package vm

import "github.com/joushou/gocnc/gcode"
import "math"
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
	moveModeNone   = iota
	moveModeRapid  = iota
	moveModeLinear = iota
	moveModeCWArc  = iota
	moveModeCCWArc = iota
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
	spindleEnabled   bool
	spindleClockwise bool
	floodCoolant     bool
	mistCoolant      bool
}

type Position struct {
	state   State
	x, y, z float64
}

type Machine struct {
	state        State
	metric       bool
	absoluteMove bool
	absoluteArc  bool
	movePlane    int
	completed    bool
	posStack     []Position
}

//
// Positioning
//

// Retrieves position from top of stack
func (vm *Machine) curPos() Position {
	return vm.posStack[len(vm.posStack)-1]
}

// Appends a position to the stack
func (vm *Machine) addPos(pos Position) {
	vm.posStack = append(vm.posStack, pos)
}

// Calculates the absolute position of the given statement, including optional I, J, K parameters
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

	if !vm.absoluteMove {
		newX, newY, newZ = pos.x+newX, pos.y+newY, pos.z+newZ
	}

	if !vm.absoluteArc {
		newI, newJ, newK = pos.x+newI, pos.y+newJ, pos.z+newK
	}
	return newX, newY, newZ, newI, newJ, newK
}

func (vm *Machine) positioning(stmt Statement) {
	newX, newY, newZ, _, _, _ := vm.calcPos(stmt)
	vm.addPos(Position{vm.state, newX, newY, newZ})
}

// Calculates an approximate arc from the provided statement
func (vm *Machine) approximateArc(stmt Statement, pointDistance float64, ignoreRadiusErrors bool) {
	startPos := vm.curPos()
	startX, startY, startZ := startPos.x, startPos.y, startPos.z
	endX, endY, endZ, endI, endJ, endK := vm.calcPos(stmt)

	clockwise := (vm.state.moveMode == moveModeCWArc)

	vm.state.moveMode = moveModeLinear

	var add func(x, y, z float64)

	switch vm.movePlane {
	case planeXY:
		add = func(x, y, z float64) {
			vm.positioning(Statement{'X': x, 'Y': y, 'Z': z})
		}
	case planeXZ:
		startY, startZ = startZ, startY
		endY, endZ = endZ, endY
		endJ, endK = endK, endJ
		add = func(x, y, z float64) {
			vm.positioning(Statement{'X': x, 'Y': z, 'Z': y})
		}
	case planeYZ:
		startX, startZ = startZ, startX
		endX, endZ = endZ, endX
		endI, endK = endK, endI
		add = func(x, y, z float64) {
			vm.positioning(Statement{'X': z, 'Y': y, 'Z': x})
		}
	}

	radius1 := math.Sqrt(math.Pow(endI-startX*2, 2) + math.Pow(endJ-startY*2, 2))
	radius2 := math.Sqrt(math.Pow(endI-endX, 2) + math.Pow(endJ-endX, 2))

	if math.Abs((radius2-radius1)/radius1) > 0.01 && !ignoreRadiusErrors {
		panic(fmt.Sprintf("Radius deviation of %f percent", math.Abs((radius2-radius1)/radius1)*100))
	}

	theta1 := math.Atan2((startY - endJ), (startX - endI))
	theta2 := math.Atan2((endY - endJ), (endX - endI))
	tRange := 0.0
	if clockwise {
		tRange = math.Abs(theta2 - theta1)
	} else {
		tRange = 2*math.Pi - math.Abs(theta2-theta1)
	}

	// Approximate if radii are not equal..
	arcLen := tRange * math.Sqrt(math.Pow((radius1+radius2)/2, 2)+math.Pow((endZ-startZ)/tRange, 2))
	steps := int(arcLen / pointDistance)

	angle := 0.0
	for i := 0; i <= steps; i++ {
		if clockwise {
			angle = theta1 - math.Abs(theta2-theta1)/float64(steps)*float64(i)
		} else {
			angle = theta1 + (2*math.Pi-math.Abs(theta2-theta1))/float64(steps)*float64(i)
		}
		localRadius := radius1 + (radius2-radius1)/float64(steps)*float64(i)
		x, y := endI+localRadius*math.Cos(angle), endJ+localRadius*math.Sin(angle)
		z := startZ + (endZ-startZ)/float64(steps)*float64(i)
		add(x, y, z)
	}
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
	if g, ok := stmt['G']; ok {
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
		if g <= 0 {
			return errors.New("Feedrate must be greater than zero")
		}
		vm.state.feedrate = g
	}

	// S-codes
	if g, ok := stmt['S']; ok {
		if g < 0 {
			return errors.New("Spindle speed must be greater than or equal to zero")
		}
		vm.state.spindleSpeed = g
	}

	// X, Y, Z, I, J, K, P
	_, hasX := stmt['X']
	_, hasY := stmt['Y']
	_, hasZ := stmt['Z']
	if hasX || hasY || hasZ {
		if vm.state.moveMode == moveModeCWArc || vm.state.moveMode == moveModeCCWArc {
			vm.approximateArc(stmt, 0.1, false)
		} else if vm.state.moveMode == moveModeLinear || vm.state.moveMode == moveModeRapid {
			vm.positioning(stmt)
		} else {
			return errors.New("Move attempted without an active move mode")
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

//
// Initialize VM state
//
func (vm *Machine) Init() {
	vm.posStack = append(vm.posStack, Position{})
	vm.metric = true
	vm.absoluteMove = true
	vm.absoluteArc = true
	vm.movePlane = planeXY
}

//
// Process an AST
//
func (vm *Machine) Process(doc *gcode.Document) (err error) {
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
		if err := vm.run(stmt); err != nil {
			return err
		}
	}
	vm.finalize()
	return
}
