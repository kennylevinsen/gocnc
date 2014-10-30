package vm

import "github.com/joushou/gocnc/gcode"
import "math"

//
// The CNC interpreter/"vm"
//
// It currently supports:
//
//   G00 - rapid move
//   G01 - linear move
//   G02 - cw arc
//   G03 - ccw arc
//   G20 - imperial mode
//   G21 - metric mode
//   G80 - cancel mode (?)
//   G90 - absolute
//   G91 - relative
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

type Statement map[rune]float64

//
// State structs
//

const (
	moveModeInitial = iota
	moveModeRapid   = iota
	moveModeLinear  = iota
	moveModeCWArc   = iota
	moveModeCCWArc  = iota
)

const (
	planeXY = iota
	planeXZ = iota
	planeYZ = iota
)

const (
	vmModeNone        = iota
	vmModePositioning = iota
)

type State struct {
	feedrate         float64
	spindleSpeed     float64
	moveMode         int
	movePlane        int
	spindleEnabled   bool
	spindleClockwise bool
	floodCoolant     bool
	mistCoolant      bool
}

func (s *State) Equal(o *State) bool {
	return (s.feedrate == o.feedrate &&
		s.spindleSpeed == o.spindleSpeed &&
		s.moveMode == o.moveMode &&
		s.movePlane == o.movePlane &&
		s.spindleEnabled == o.spindleEnabled &&
		s.spindleClockwise == o.spindleClockwise &&
		s.floodCoolant == o.floodCoolant &&
		s.mistCoolant == o.mistCoolant)
}

type Position struct {
	state   State
	x, y, z float64
}

type Machine struct {
	state     State
	mode      int
	metric    bool
	absolute  bool
	completed bool
	posStack  []Position
}

//
// Positioning
//
func (vm *Machine) curPos() Position {
	return vm.posStack[len(vm.posStack)-1]
}

func (vm *Machine) addPos(pos Position) {
	vm.posStack = append(vm.posStack, pos)
}

func (vm *Machine) calcPos(stmt Statement) (newX, newY, newZ, newI, newJ, newK float64) {
	pos := vm.curPos()
	var ok bool

	if newX, ok = stmt['X']; !ok {
		newX = pos.x
	} else if !vm.metric {
		newX *= 25.4
	}

	if newY, ok = stmt['Y']; !ok {
		newY = pos.y
	} else if !vm.metric {
		newY *= 25.4
	}

	if newZ, ok = stmt['Z']; !ok {
		newZ = pos.z
	} else if !vm.metric {
		newZ *= 25.4
	}

	newI = stmt['I']
	newJ = stmt['J']
	newK = stmt['K']

	if !vm.metric {
		newI, newJ, newK = newI*25.4, newJ*25.4, newZ*25.4
	}

	if !vm.absolute {
		newX, newY, newZ = newX+pos.x, newY+pos.y, newZ+pos.z
	}
	return newX, newY, newZ, newI, newJ, newK
}

func (vm *Machine) positioning(stmt Statement) {
	newX, newY, newZ, _, _, _ := vm.calcPos(stmt)
	vm.addPos(Position{vm.state, newX, newY, newZ})
}

//
// Approximate arc
//
func (vm *Machine) approximateArc(stmt Statement, pointDistance float64) {
	startPos := vm.curPos()
	endX, endY, endZ, endI, endJ, _ := vm.calcPos(stmt)

	clockWise := (vm.state.moveMode == moveModeCWArc)

	vm.state.moveMode = moveModeLinear

	if vm.state.movePlane == planeXY {
		cX, cY := endI+startPos.x, endJ+startPos.y
		radius := math.Sqrt(math.Pow(endI-startPos.x, 2) + math.Pow(endJ-startPos.y, 2))
		theta1 := math.Atan2((startPos.y - cY), (startPos.x - cX))
		theta2 := math.Atan2((endY - cY), (endX - cX))

		tRange := math.Abs(theta2 - theta1)
		arcLen := tRange * math.Sqrt(math.Pow(radius, 2)+math.Pow((endZ-startPos.z)/tRange, 2))
		steps := int(arcLen / pointDistance)

		angle := 0.0
		for i := 0; i <= steps; i++ {
			if clockWise {
				angle = theta1 + (theta2-theta1-2*math.Pi)/float64(steps)*float64(i)
			} else {
				angle = theta1 + (theta2-theta1)/float64(steps)*float64(i)
			}
			x, y := cX+radius*math.Cos(angle), cY+radius*math.Sin(angle)
			z := startPos.z + endZ/float64(steps)*float64(i)

			vm.positioning(Statement{'X': x, 'Y': y, 'Z': z})
		}

	} else if vm.state.movePlane == planeXZ {

	} else if vm.state.movePlane == planeYZ {

	}
}

//
// Dispatch
//
func (vm *Machine) run(stmt Statement) {
	if vm.completed {
		// A stop had previously been issued
		return
	}

	// G-codes
	if g, ok := stmt['G']; ok {
		switch g {
		case 0:
			vm.mode = vmModePositioning
			vm.state.moveMode = moveModeRapid
		case 1:
			vm.mode = vmModePositioning
			vm.state.moveMode = moveModeLinear
		case 2:
			vm.mode = vmModePositioning
			vm.state.moveMode = moveModeCWArc
		case 3:
			vm.mode = vmModePositioning
			vm.state.moveMode = moveModeCCWArc
		case 17:
			vm.state.movePlane = planeXY
		case 18:
			vm.state.movePlane = planeXZ
		case 19:
			vm.state.movePlane = planeYZ
		case 20:
			vm.metric = false
		case 21:
			vm.metric = true
		case 80:
			vm.mode = vmModeNone
		case 90:
			vm.absolute = true
		case 91:
			vm.absolute = false
		}
	}

	// M-codes
	if g, ok := stmt['M']; ok {
		switch g {
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
		case 7:
			vm.state.mistCoolant = true
		case 8:
			vm.state.floodCoolant = true
		case 9:
			vm.state.mistCoolant = false
			vm.state.floodCoolant = false
		case 30:
			vm.completed = true
		}
	}

	// F-codes
	if g, ok := stmt['F']; ok {
		if !vm.metric {
			g *= 25.4
		}
		vm.state.feedrate = g
	}

	// S-codes
	if g, ok := stmt['S']; ok {
		vm.state.spindleSpeed = g
	}

	// X, Y, Z, I, J, K, P
	_, hasX := stmt['X']
	_, hasY := stmt['Y']
	_, hasZ := stmt['Z']
	if (hasX || hasY || hasZ) && vm.mode == vmModePositioning {
		if vm.state.moveMode == moveModeCWArc || vm.state.moveMode == moveModeCCWArc {
			vm.approximateArc(stmt, 0.1)
		} else {
			vm.positioning(stmt)
		}
	}
}

//
// Initialize VM state
//
func (vm *Machine) Init() {
	vm.posStack = append(vm.posStack, Position{})
	vm.metric = true
	vm.absolute = true
}

//
// Process an AST
//
func (vm *Machine) Process(doc *gcode.Document) {
	for _, b := range doc.Blocks {
		if b.BlockDelete {
			continue
		}

		stmt := make(Statement)
		for _, n := range b.Nodes {
			if word, ok := n.(*gcode.Word); ok {
				stmt[word.Address] = word.Command
			}
		}
		vm.run(stmt)
	}
}
