package vm

import "github.com/joushou/gocnc/gcode"
import "fmt"
import "errors"

//
// The CNC interpreter/"vm"
//
// It currently supports:
//
//   G00   - rapid move
//   G01   - linear move
//   G02   - cw arc
//   G03   - ccw arc
//   G17   - xy arc plane
//   G18   - xz arc plane
//   G19   - yz arc plane
//   G20   - imperial mode
//   G21   - metric mode
//   G80   - cancel mode (?)
//   G90   - absolute
//   G90.1 - absolute arc
//   G91   - relative
//   G91.1 - relative arc
//   G93   - inverse feed mode
//   G94   - units per minute feed mode
//   G95   - units per revolution feed mode
//
//   M02 - end of program
//   M03 - spindle enable clockwise
//   M04 - spindle enable counterclockwise
//   M05 - spindle disable
//   M07 - mist coolant enable
//   M08 - flood coolant enable
//   M09 - coolant disable
//   M30 - end of program
//
//   F - feedrate
//   S - spindle speed
//   P - parameter
//   X, Y, Z - cartesian movement
//   I, J, K - arc center definition

//
// TODO
//
//   TESTS?! At least one per code!
//
//   Handle multiple G/M codes on the same line (slice of pairs instead of map)
//   Split G/M handling out of the run function
//   Handle G/M-code priority properly
//   Better comments
//   Implement various canned cycles
//   Variables (basic support?)
//   Subroutines
//   Incremental axes
//   A, B, C axes
//
// State structs and constants
//

// Constants for move modes
const (
	MoveModeNone   = iota
	MoveModeRapid  = iota
	MoveModeLinear = iota
	MoveModeCWArc  = iota
	MoveModeCCWArc = iota
)

// Constants for plane selection
const (
	PlaneXY = iota
	PlaneXZ = iota
	PlaneYZ = iota
)

// Constants for feedrate mode
const (
	FeedModeUnitsMin = iota
	FeedModeUnitsRev = iota
	FeedModeInvTime  = iota
)

// Constants for cutter compensation mode
const (
	CutCompModeNone  = iota
	CutCompModeOuter = iota
	CutCompModeInner = iota
)

// Move state
type State struct {
	Feedrate           float64
	SpindleSpeed       float64
	MoveMode           int
	FeedMode           int
	SpindleEnabled     bool
	SpindleClockwise   bool
	FloodCoolant       bool
	MistCoolant        bool
	Tool               int
	CutterCompensation int
}

// Position and state
type Position struct {
	State   State
	X, Y, Z float64
}

// Machine state and settings
type Machine struct {
	State            State
	Completed        bool
	Metric           bool
	AbsoluteMove     bool
	AbsoluteArc      bool
	MovePlane        int
	NextTool         int
	MaxArcDeviation  float64
	MinArcLineLength float64
	Tolerance        float64
	Positions        []Position
}

//
// Struct and helper functions to aid execution
//
type statement []*gcode.Word

func (stmt statement) get(address rune) (res float64, err error) {
	found := false
	for _, m := range stmt {
		if m.Address == address {
			if found {
				return res, errors.New(fmt.Sprintf("Multiple instances of address '%c' in block", address))
			}
			found = true
			res = m.Command
		}
	}
	if !found {
		return res, errors.New(fmt.Sprintf("'%c' not found in block", address))
	}
	return res, nil
}

func (stmt statement) getDefault(address rune, def float64) (res float64) {
	res, err := stmt.get(address)
	if err != nil {
		return def
	}
	return res
}

func (stmt statement) getAll(address rune) (res []float64) {
	for _, m := range stmt {
		if m.Address == address {
			res = append(res, m.Command)
		}
	}
	return res
}

func (stmt statement) includes(addresses ...rune) (res bool) {
	for _, m := range addresses {
		_, err := stmt.get(m)
		if err == nil {
			return true
		}
	}
	return false
}

func (stmt statement) hasWord(address rune, command float64) (res bool) {
	for _, m := range stmt {
		if m.Address == address && m.Command == command {
			return true
		}
	}
	return false
}

//
// Dispatch
//

func (vm *Machine) run(stmt statement) (err error) {
	if vm.Completed {
		// A stop had previously been issued
		return
	}

	defer func() {
		if r := recover(); r != nil {
			err = errors.New(fmt.Sprintf("%s", r))
		}
	}()

	// G-codes
	for _, g := range stmt.getAll('G') {
		switch g {
		case 0:
			vm.State.MoveMode = MoveModeRapid
		case 1:
			vm.State.MoveMode = MoveModeLinear
		case 2:
			vm.State.MoveMode = MoveModeCWArc
		case 3:
			vm.State.MoveMode = MoveModeCCWArc
		case 4:
			// TODO handle dwell?
		case 17:
			vm.MovePlane = PlaneXY
		case 18:
			vm.MovePlane = PlaneXZ
		case 19:
			vm.MovePlane = PlaneYZ
		case 20:
			vm.Metric = false
		case 21:
			vm.Metric = true
		case 40:
			vm.State.CutterCompensation = CutCompModeNone
		case 41:
			vm.State.CutterCompensation = CutCompModeOuter
		case 42:
			vm.State.CutterCompensation = CutCompModeInner
		case 64:
			// TODO Handle naive cam tolerance
			vm.Tolerance = stmt.getDefault('P', vm.Tolerance)
		case 80:
			vm.State.MoveMode = MoveModeNone
		case 90:
			vm.AbsoluteMove = true
		case 90.1:
			vm.AbsoluteArc = true
		case 91:
			vm.AbsoluteMove = false
		case 91.1:
			vm.AbsoluteArc = false
		case 93:
			vm.State.FeedMode = FeedModeInvTime
		case 94:
			vm.State.FeedMode = FeedModeUnitsMin
		case 95:
			vm.State.FeedMode = FeedModeUnitsRev
		default:
			panic(fmt.Sprintf("G%g not supported", g))
		}
	}

	for _, t := range stmt.getAll('T') {
		if t < 0 {
			panic("Tool must be non-negative")
		}
		vm.NextTool = int(t)
	}

	// M-codes
	for _, m := range stmt.getAll('M') {
		switch m {
		case 2:
			vm.Completed = true
		case 3:
			vm.State.SpindleEnabled = true
			vm.State.SpindleClockwise = true
		case 4:
			vm.State.SpindleEnabled = true
			vm.State.SpindleClockwise = false
		case 5:
			vm.State.SpindleEnabled = false
		case 6:
			vm.State.Tool = vm.NextTool
		case 7:
			vm.State.MistCoolant = true
		case 8:
			vm.State.FloodCoolant = true
		case 9:
			vm.State.MistCoolant = false
			vm.State.FloodCoolant = false
		case 30:
			vm.Completed = true
		default:
			panic(fmt.Sprintf("M%g not supported", m))
		}
	}

	// F-codes
	for _, f := range stmt.getAll('F') {
		if !vm.Metric {
			f *= 25.4
		}
		if f <= 0 {
			panic("Feedrate must be greater than zero")
		}
		vm.State.Feedrate = f
	}

	// S-codes
	for _, s := range stmt.getAll('S') {
		if s < 0 {
			panic("Spindle speed must be greater than or equal to zero")
		}
		vm.State.SpindleSpeed = s
	}

	if stmt.includes('A', 'B', 'C', 'U', 'V', 'W') {
		panic("Only X, Y and Z axes are supported")
	}

	if stmt.includes('X', 'Y', 'Z') {
		if vm.State.MoveMode == MoveModeCWArc || vm.State.MoveMode == MoveModeCCWArc {
			vm.approximateArc(stmt)
		} else if vm.State.MoveMode == MoveModeLinear || vm.State.MoveMode == MoveModeRapid {
			vm.positioning(stmt)
		} else {
			panic("Move attempted without an active move mode")
		}
	}

	return nil
}

// Ensure that machine state is correct after execution
func (vm *Machine) finalize() {
	if vm.State != vm.curPos().State {
		vm.State.MoveMode = MoveModeNone
		vm.addPos(Position{State: vm.State})
	}
}

// Process AST
func (vm *Machine) Process(doc *gcode.Document) (err error) {
	for idx, b := range doc.Blocks {
		if b.BlockDelete {
			continue
		}

		stmt := make(statement, 0)
		for _, n := range b.Nodes {
			if word, ok := n.(*gcode.Word); ok {
				stmt = append(stmt, word)
			}
		}
		if err := vm.run(stmt); err != nil {
			return errors.New(fmt.Sprintf("line %d: %s", idx+1, err))
		}
	}
	vm.finalize()
	return
}

// Initialize the VM to sane default values
func (vm *Machine) Init() {
	vm.Positions = append(vm.Positions, Position{})
	vm.Metric = true
	vm.AbsoluteMove = true
	vm.AbsoluteArc = false
	vm.MovePlane = PlaneXY
	vm.MaxArcDeviation = 0.002
	vm.MinArcLineLength = 0.01
	vm.Tolerance = 0.001
}

// Dump position in (sort of) human readable format
func (m *Position) Dump() {
	switch m.State.MoveMode {
	case MoveModeNone:
		fmt.Printf("Null move\n")
	case MoveModeRapid:
		fmt.Printf("Rapid move\n")
	case MoveModeLinear:
		fmt.Printf("Linear move\n")
	case MoveModeCWArc:
		fmt.Printf("Clockwise arc\n")
	case MoveModeCCWArc:
		fmt.Printf("Counterclockwise arc\n")
	}
	fmt.Printf("   Feedrate: %g\n", m.State.Feedrate)
	fmt.Printf("   Spindle: %t, clockwise: %t, speed: %g\n", m.State.SpindleEnabled, m.State.SpindleClockwise, m.State.SpindleSpeed)
	fmt.Printf("   Mist coolant: %t, flood coolant: %t\n", m.State.MistCoolant, m.State.FloodCoolant)
	fmt.Printf("   X: %f, Y: %f, Z: %f\n", m.X, m.Y, m.Z)
}

// Dumps the entire machine
func (vm *Machine) Dump() {
	for _, m := range vm.Positions {
		m.Dump()
	}
}
