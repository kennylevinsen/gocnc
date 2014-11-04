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

type Statement []*gcode.Word

func (stmt Statement) get(address rune) (res float64, err error) {
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

func (stmt Statement) getDefault(address rune, def float64) (res float64) {
	res, err := stmt.get(address)
	if err == nil {
		return def
	}
	return res
}

func (stmt Statement) getAll(address rune) (res []float64) {
	for _, m := range stmt {
		if m.Address == address {
			res = append(res, m.Command)
		}
	}
	return res
}

func (stmt Statement) includes(addresses ...rune) (res bool) {
	for _, m := range addresses {
		_, err := stmt.get(m)
		if err == nil {
			return true
		}
	}
	return false
}

func (stmt Statement) hasWord(address rune, command float64) (res bool) {
	for _, m := range stmt {
		if m.Address == address && m.Command == command {
			return true
		}
	}
	return false
}

//
// State structs and constants
//

// Constants for move modes
const (
	moveModeNone   = iota
	moveModeRapid  = iota
	moveModeLinear = iota
	moveModeCWArc  = iota
	moveModeCCWArc = iota
)

// Constants for plane selection
const (
	planeXY = iota
	planeXZ = iota
	planeYZ = iota
)

// Constants for feedrate mode
const (
	feedModeUnitsMin = iota
	feedModeUnitsRev = iota
	feedModeInvTime  = iota
)

// Constants for cutter compensation mode
const (
	cutCompModeNone  = iota
	cutCompModeOuter = iota
	cutCompModeInner = iota
)

// Move state
type State struct {
	feedrate           float64
	spindleSpeed       float64
	moveMode           int
	feedMode           int
	spindleEnabled     bool
	spindleClockwise   bool
	floodCoolant       bool
	mistCoolant        bool
	tool               int
	nextTool           int
	cutterCompensation int
}

// Position and state
type Position struct {
	state   State
	x, y, z float64
}

// Machine state and settings
type Machine struct {
	state            State
	completed        bool
	metric           bool
	absoluteMove     bool
	absoluteArc      bool
	movePlane        int
	MaxArcDeviation  float64
	MinArcLineLength float64
	Tolerance        float64
	Positions        []Position
}

//
// Dispatch
//

func (vm *Machine) run(stmt Statement) (err error) {
	if vm.completed {
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
			vm.state.moveMode = moveModeRapid
		case 1:
			vm.state.moveMode = moveModeLinear
		case 2:
			vm.state.moveMode = moveModeCWArc
		case 3:
			vm.state.moveMode = moveModeCCWArc
		case 17:
			vm.movePlane = planeXY
		case 18:
			vm.movePlane = planeXZ
		case 19:
			vm.movePlane = planeYZ
		case 20:
			vm.metric = false
		case 21:
			vm.metric = true
		case 40:
			vm.state.cutterCompensation = cutCompModeNone
		case 41:
			vm.state.cutterCompensation = cutCompModeOuter
		case 42:
			vm.state.cutterCompensation = cutCompModeInner
		case 80:
			vm.state.moveMode = moveModeNone
		case 90:
			vm.absoluteMove = true
		case 90.1:
			vm.absoluteArc = true
		case 91:
			vm.absoluteMove = false
		case 91.1:
			vm.absoluteArc = false
		case 93:
			vm.state.feedMode = feedModeInvTime
		case 94:
			vm.state.feedMode = feedModeUnitsMin
		case 95:
			vm.state.feedMode = feedModeUnitsRev
		default:
			panic(fmt.Sprintf("G%f not supported", g))
		}
	}

	for _, t := range stmt.getAll('T') {
		if t < 0 {
			panic("T-word (tool select) must be non-negative")
		}
		vm.state.nextTool = int(t)
	}

	// M-codes
	for _, m := range stmt.getAll('M') {
		switch m {
		case 2:
			vm.completed = true
		case 3:
			vm.state.spindleEnabled = true
			vm.state.spindleClockwise = true
		case 4:
			vm.state.spindleEnabled = true
			vm.state.spindleClockwise = false
		case 5:
			vm.state.spindleEnabled = false
		case 6:
			vm.state.tool = vm.state.nextTool
		case 7:
			vm.state.mistCoolant = true
		case 8:
			vm.state.floodCoolant = true
		case 9:
			vm.state.mistCoolant = false
			vm.state.floodCoolant = false
		case 30:
			vm.completed = true
		default:
			panic(fmt.Sprintf("M%.1f not supported", m))
		}
	}

	// F-codes
	for _, f := range stmt.getAll('F') {
		if !vm.metric {
			f *= 25.4
		}
		if f <= 0 {
			panic("Feedrate must be greater than zero")
		}
		vm.state.feedrate = f
	}

	// S-codes
	for _, s := range stmt.getAll('S') {
		if s < 0 {
			panic("Spindle speed must be greater than or equal to zero")
		}
		vm.state.spindleSpeed = s
	}

	// X, Y, Z, I, J, K, P
	hasPositioning := stmt.includes('X', 'Y', 'Z')
	if hasPositioning {
		if vm.state.moveMode == moveModeCWArc || vm.state.moveMode == moveModeCCWArc {
			vm.approximateArc(stmt)
		} else if vm.state.moveMode == moveModeLinear || vm.state.moveMode == moveModeRapid {
			vm.positioning(stmt)
		} else {
			panic("Move attempted without an active move mode")
		}
	}

	return nil
}

// Ensure that machine state is correct after execution
func (vm *Machine) finalize() {
	if vm.state != vm.curPos().state {
		vm.state.moveMode = moveModeNone
		vm.addPos(Position{state: vm.state})
	}
}

// Process AST
func (vm *Machine) Process(doc *gcode.Document) (err error) {
	for _, b := range doc.Blocks {
		if b.BlockDelete {
			continue
		}

		stmt := make(Statement, 0)
		for _, n := range b.Nodes {
			if word, ok := n.(*gcode.Word); ok {
				stmt = append(stmt, word)
			}
		}
		if err := vm.run(stmt); err != nil {
			return err
		}
	}
	vm.finalize()
	return
}

// Initialize the VM to sane default values
func (vm *Machine) Init() {
	vm.Positions = append(vm.Positions, Position{})
	vm.metric = true
	vm.absoluteMove = true
	vm.absoluteArc = false
	vm.movePlane = planeXY
	vm.MaxArcDeviation = 0.002
	vm.MinArcLineLength = 0.01
	vm.Tolerance = 0.001
}
