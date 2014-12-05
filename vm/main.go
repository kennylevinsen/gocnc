package vm

import "github.com/joushou/gocnc/gcode"
import "github.com/joushou/gocnc/vector"
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
//   G40   - cutter compensation
//   G41   - cutter compensation
//   G42   - cutter compensation
//   G64   - tolerance
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
//   M06 - toolchange
//   M07 - mist coolant enable
//   M08 - flood coolant enable
//   M09 - coolant disable
//   M30 - end of program
//
//   F - feedrate
//   S - spindle speed
//   P - parameter
//   T - tool
//   X, Y, Z - cartesian movement
//   I, J, K - arc center definition
//
// Notes:
//   Dwell (G04) is ignored
//   Tolerance (G64) is ignored
//   Cutter compensation is just passed to machine
//

//
// TODO
//
//   TESTS?! At least one per code!
//   Error cases (Extra/Missing arguments, etc.)
//   Execution order
//   Modal groups
//   Better comments
//   Implement various canned cycles
//   Variables (basic support?)
//   Subroutines
//   A, B, C axes
//

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

func (p Position) Vector() vector.Vector {
	return vector.Vector{p.X, p.Y, p.Z}
}

// Machine state and settings
type Machine struct {
	State            State
	Completed        bool
	Imperial         bool
	AbsoluteMove     bool
	AbsoluteArc      bool
	MovePlane        int
	NextTool         int
	MaxArcDeviation  float64
	MinArcLineLength float64
	Tolerance        float64
	StoredPos1       vector.Vector
	StoredPos2       vector.Vector
	Positions        []Position
}

//
// Dispatch
//
/*
func (vm *Machine) handleG(stmt gcode.Block) {
	for _, g := range stmt.GetAllWords('G') {
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
			vm.Imperial = true
		case 21:
			vm.Imperial = false
		case 28:
			// HACK
		case 40:
			vm.State.CutterCompensation = CutCompModeNone
		case 41:
			vm.State.CutterCompensation = CutCompModeOuter
		case 42:
			vm.State.CutterCompensation = CutCompModeInner
		case 64:
			// TODO I presume this is safe to ignore?
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
}

func (vm *Machine) handleM(stmt gcode.Block) {
	for _, m := range stmt.GetAllWords('M') {
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
}

func (vm *Machine) handleT(stmt gcode.Block) {
	for _, t := range stmt.GetAllWords('T') {
		if t < 0 {
			panic("Tool must be non-negative")
		}
		vm.NextTool = int(t)
	}
}

func (vm *Machine) handleF(stmt gcode.Block) {
	for _, f := range stmt.GetAllWords('F') {
		if vm.Imperial {
			f *= 25.4
		}
		if f <= 0 {
			panic("Feedrate must be greater than zero")
		}
		vm.State.Feedrate = f
	}
}

*/

func (vm *Machine) handleS(stmt gcode.Block) {
	for _, s := range stmt.GetAllWords('S') {
		if s < 0 {
			panic("Spindle speed must be greater than or equal to zero")
		}
		vm.State.SpindleSpeed = s
	}
}

func (vm *Machine) feedRateMode(stmt *gcode.Block) {
	if w, err := stmt.GetModalGroup("feedRateModeGroup"); err == nil {
		if w != nil {
			if w.Address != 'G' {
				panic(fmt.Sprintf("GetModalGroup returned unexpected command %s", w.Export(-1)))
			}

			oldMode := vm.State.FeedMode
			switch w.Command {
			case 93:
				vm.State.FeedMode = FeedModeInvTime
			case 94:
				vm.State.FeedMode = FeedModeUnitsMin
			case 95:
				vm.State.FeedMode = FeedModeUnitsRev
			default:
				panic(fmt.Sprintf("GetModalGroup returned unexpected command %s", w.Export(-1)))
			}
			if vm.State.FeedMode != oldMode {
				// Ensure that feedrate is cleared
				vm.State.Feedrate = 0
			}
			stmt.Remove(w)
		}
	} else {
		panic(fmt.Sprintf("%s", err))
	}

}

func (vm *Machine) feedRate(stmt *gcode.Block) {
	if val, err := stmt.GetWord('F'); err == nil {
		vm.State.Feedrate = val
		stmt.RemoveAddress('F')
	} else if vm.State.FeedMode == FeedModeInvTime {
		vm.State.Feedrate = -1
	}
}

func (vm *Machine) spindleSpeed(stmt *gcode.Block) {
	if val, err := stmt.GetWord('S'); err == nil {
		vm.State.SpindleSpeed = val
		stmt.RemoveAddress('S')
	}
}

func (vm *Machine) nextTool(stmt *gcode.Block) {
	if val, err := stmt.GetWord('T'); err == nil {
		vm.NextTool = int(val)
		stmt.RemoveAddress('T')
	}
}

func (vm *Machine) toolChange(stmt *gcode.Block) {
	if w, err := stmt.GetModalGroup("toolChangeGroup"); err == nil {
		if w != nil {
			if w.Address != 'M' {
				panic(fmt.Sprintf("GetModalGroup returned unexpected command %s", w.Export(-1)))
			}

			switch w.Command {
			case 6:
				if vm.NextTool == -1 {
					panic("Toolchange attempted without a defined tool")
				}
				vm.State.Tool = vm.NextTool
			default:
				panic(fmt.Sprintf("GetModalGroup returned unexpected command %s", w.Export(-1)))
			}
			stmt.Remove(w)
		}
	} else {
		panic(fmt.Sprintf("%s", err))
	}

}

func (vm *Machine) setSpindle(stmt *gcode.Block) {
	if w, err := stmt.GetModalGroup("spindleGroup"); err == nil {
		if w != nil {
			if w.Address != 'M' {
				panic(fmt.Sprintf("GetModalGroup returned unexpected command %s", w.Export(-1)))
			}

			switch w.Command {
			case 3:
				vm.State.SpindleEnabled = true
				vm.State.SpindleClockwise = true
			case 4:
				vm.State.SpindleEnabled = true
				vm.State.SpindleClockwise = false
			case 5:
				vm.State.SpindleEnabled = false
			default:
				panic(fmt.Sprintf("GetModalGroup returned unexpected command %s", w.Export(-1)))
			}
			stmt.Remove(w)
		}
	} else {
		panic(fmt.Sprintf("%s", err))
	}

}

func (vm *Machine) setCoolant(stmt *gcode.Block) {
	// TODO Handle M7 and M8 on same line
	if w, err := stmt.GetModalGroup("coolantGroup"); err == nil {
		if w != nil {
			if w.Address != 'M' {
				panic(fmt.Sprintf("GetModalGroup returned unexpected command %s", w.Export(-1)))
			}

			switch w.Command {
			case 7:
				vm.State.MistCoolant = true
			case 8:
				vm.State.FloodCoolant = true
			case 9:
				vm.State.MistCoolant = false
				vm.State.FloodCoolant = false
			default:
				panic(fmt.Sprintf("GetModalGroup returned unexpected command %s", w.Export(-1)))
			}
			stmt.Remove(w)
		}
	} else {
		panic(fmt.Sprintf("%s", err))
	}
}

func (vm *Machine) setPlane(stmt *gcode.Block) {
	if w, err := stmt.GetModalGroup("planeSelectionGroup"); err == nil {
		if w != nil {
			if w.Address != 'G' {
				panic(fmt.Sprintf("GetModalGroup returned unexpected command %s", w.Export(-1)))
			}

			switch w.Command {
			case 17:
				vm.MovePlane = PlaneXY
			case 18:
				vm.MovePlane = PlaneXZ
			case 19:
				vm.MovePlane = PlaneYZ
			default:
				panic(fmt.Sprintf("GetModalGroup returned unexpected command %s", w.Export(-1)))
			}
			stmt.Remove(w)
		}
	} else {
		panic(fmt.Sprintf("%s", err))
	}
}

func (vm *Machine) setUnits(stmt *gcode.Block) {
	if w, err := stmt.GetModalGroup("unitsGroup"); err == nil {
		if w != nil {
			if w.Address != 'G' {
				panic(fmt.Sprintf("GetModalGroup returned unexpected command %s", w.Export(-1)))
			}

			switch w.Command {
			case 20:
				vm.Imperial = true
			case 21:
				vm.Imperial = false
			default:
				panic(fmt.Sprintf("GetModalGroup returned unexpected command %s", w.Export(-1)))
			}
			stmt.Remove(w)
		}
	} else {
		panic(fmt.Sprintf("%s", err))
	}
}

func (vm *Machine) setCutterCompensation(stmt *gcode.Block) {
	if w, err := stmt.GetModalGroup("cutterCompensationModeGroup"); err == nil {
		if w != nil {
			if w.Address != 'G' {
				panic(fmt.Sprintf("GetModalGroup returned unexpected command %s", w.Export(-1)))
			}

			switch w.Command {
			case 40:
				vm.State.CutterCompensation = CutCompModeNone
			case 41:
				vm.State.CutterCompensation = CutCompModeOuter
			case 42:
				vm.State.CutterCompensation = CutCompModeInner
			default:
				panic(fmt.Sprintf("GetModalGroup returned unexpected command %s", w.Export(-1)))
			}
			stmt.Remove(w)
		}
	} else {
		panic(fmt.Sprintf("%s", err))
	}
}

func (vm *Machine) setCoordinateSystem(stmt *gcode.Block) {
	// TODO Implement coordinate system offsets!
	if w, err := stmt.GetModalGroup("coordinateSystemGroup"); err == nil {
		if w != nil {
			if w.Address != 'G' {
				panic(fmt.Sprintf("GetModalGroup returned unexpected command %s", w.Export(-1)))
			}

			switch w.Command {
			case 53:
				// Ignore!
			case 54:
				// Ignore!
			default:
				panic(fmt.Sprintf("GetModalGroup returned unexpected command %s", w.Export(-1)))
			}
			stmt.Remove(w)
		}
	} else {
		panic(fmt.Sprintf("%s", err))
	}
}

func (vm *Machine) setDistanceMode(stmt *gcode.Block) {
	if w, err := stmt.GetModalGroup("distanceModeGroup"); err == nil {
		if w != nil {
			if w.Address != 'G' {
				panic(fmt.Sprintf("GetModalGroup returned unexpected command %s", w.Export(-1)))
			}

			switch w.Command {
			case 90:
				vm.AbsoluteMove = true
			case 91:
				vm.AbsoluteMove = false
			default:
				panic(fmt.Sprintf("GetModalGroup returned unexpected command %s", w.Export(-1)))
			}
			stmt.Remove(w)
		}
	} else {
		panic(fmt.Sprintf("%s", err))
	}
}

func (vm *Machine) nonModals(stmt *gcode.Block) {
	if w, err := stmt.GetModalGroup("nonModalGroup"); err == nil {
		if w != nil {
			if w.Address != 'G' {
				panic(fmt.Sprintf("GetModalGroup returned unexpected command %s", w.Export(-1)))
			}

			switch w.Command {
			case 4:
				// TODO Handle!!!
			case 28:
				oldMode := vm.State.MoveMode
				vm.State.MoveMode = MoveModeRapid
				if stmt.IncludesOneOf('X', 'Y', 'Z') {
					vm.move(*stmt)
					stmt.RemoveAddress('X', 'Y', 'Z')
				}
				vm.addPos(Position{vm.State, vm.StoredPos1.X, vm.StoredPos1.Y, vm.StoredPos1.Z})
				vm.State.MoveMode = oldMode
			case 28.1:
				pos := vm.curPos()
				vm.StoredPos1 = pos.Vector()
			case 30:
				oldMode := vm.State.MoveMode
				vm.State.MoveMode = MoveModeRapid
				if stmt.IncludesOneOf('X', 'Y', 'Z') {
					vm.move(*stmt)
					stmt.RemoveAddress('X', 'Y', 'Z')
				}
				vm.addPos(Position{vm.State, vm.StoredPos2.X, vm.StoredPos2.Y, vm.StoredPos2.Z})
				vm.State.MoveMode = oldMode
			case 30.1:
				pos := vm.curPos()
				vm.StoredPos2 = pos.Vector()
			default:
				panic(fmt.Sprintf("GetModalGroup returned unexpected command %s", w.Export(-1)))
			}
			stmt.Remove(w)
		}
	} else {
		panic(fmt.Sprintf("%s", err))
	}
}

func (vm *Machine) setMove(stmt *gcode.Block) {
	if w, err := stmt.GetModalGroup("motionGroup"); err == nil {
		if w != nil {
			if w.Address != 'G' {
				panic(fmt.Sprintf("GetModalGroup returned unexpected command %s", w.Export(-1)))
			}

			switch w.Command {
			case 0:
				vm.State.MoveMode = MoveModeRapid
			case 1:
				vm.State.MoveMode = MoveModeLinear
			case 2:
				vm.State.MoveMode = MoveModeCWArc
			case 3:
				vm.State.MoveMode = MoveModeCCWArc
			default:
				panic(fmt.Sprintf("GetModalGroup returned unexpected command %s", w.Export(-1)))
			}
			stmt.Remove(w)
		}
	} else {
		panic(fmt.Sprintf("%s", err))
	}

	if stmt.IncludesOneOf('A', 'B', 'C', 'U', 'V', 'W') {
		panic("Only X, Y and Z axes are supported")
	}

	if stmt.IncludesOneOf('X', 'Y', 'Z') {
		if vm.State.FeedMode == FeedModeInvTime && vm.State.Feedrate == -1 && vm.State.MoveMode != MoveModeRapid {
			panic("Non-rapid inverse time feed mode move attempted without a set feedrate")
		}
		if vm.State.MoveMode == MoveModeCWArc || vm.State.MoveMode == MoveModeCCWArc {
			vm.arc(*stmt)
			stmt.RemoveAddress('X', 'Y', 'Z', 'I', 'J', 'K', 'P')
		} else if vm.State.MoveMode == MoveModeLinear || vm.State.MoveMode == MoveModeRapid {
			vm.move(*stmt)
			stmt.RemoveAddress('X', 'Y', 'Z')
		} else {
			panic(fmt.Sprintf("Move attempted wihtout an active move mode: %s", stmt.Export(-1)))
		}
	}
}

func (vm *Machine) setStop(stmt *gcode.Block) {
	// TODO implement
	if w, err := stmt.GetModalGroup("stoppingGroup"); err == nil {
		if w != nil {
			if w.Address != 'M' {
				panic(fmt.Sprintf("GetModalGroup returned unexpected command %s", w.Export(-1)))
			}

			switch w.Command {
			case 2:
				vm.Completed = true
			case 30:
				vm.Completed = true
			default:
				panic(fmt.Sprintf("GetModalGroup returned unexpected command %s", w.Export(-1)))
			}
			stmt.Remove(w)
		}
	} else {
		panic(fmt.Sprintf("%s", err))
	}
}

func (vm *Machine) postCheck(stmt *gcode.Block) {
	for _, w := range stmt.Nodes {
		if _, ok := w.(*gcode.Word); ok {
			panic(fmt.Sprintf("Unsupported commands left in block: %s", stmt.Export(-1)))
		}
	}
}

/*func (vm *Machine) moveGroup(smtmt gcode.Block) {
if stmt.IncludesOneOf(
*/
func (vm *Machine) run(stmt gcode.Block) (err error) {
	if vm.Completed {
		// A stop had previously been issued
		return
	}

	/*defer func() {
		if r := recover(); r != nil {
			err = errors.New(fmt.Sprintf("%s", r))
		}
	}()*/

	vm.feedRateMode(&stmt)
	vm.feedRate(&stmt)
	vm.spindleSpeed(&stmt)
	vm.nextTool(&stmt)
	vm.toolChange(&stmt)
	vm.setSpindle(&stmt)
	vm.setCoolant(&stmt)
	vm.setPlane(&stmt)
	vm.setUnits(&stmt)
	vm.setCutterCompensation(&stmt)
	vm.setCoordinateSystem(&stmt)
	vm.setDistanceMode(&stmt)
	vm.nonModals(&stmt)
	vm.setMove(&stmt)
	vm.setStop(&stmt)
	vm.postCheck(&stmt)

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

		if err := vm.run(b); err != nil {
			return errors.New(fmt.Sprintf("line %d: %s", idx+1, err))
		}
	}
	vm.finalize()
	return nil
}

// Initialize the VM to sane default values
func (vm *Machine) Init() {
	vm.Positions = append(vm.Positions, Position{})
	vm.Imperial = false
	vm.AbsoluteMove = true
	vm.AbsoluteArc = false
	vm.MovePlane = PlaneXY
	vm.MaxArcDeviation = 0.002
	vm.MinArcLineLength = 0.01
	vm.NextTool = -1
}

//
// Debug assistance
//

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
