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

// Move state
type State struct {
	feedrate         float64
	spindleSpeed     float64
	moveMode         int
	spindleEnabled   bool
	spindleClockwise bool
	floodCoolant     bool
	mistCoolant      bool
}

// Position and state
type Position struct {
	state   State
	x, y, z float64
}

// Machine state and settings
type Machine struct {
	state            State
	metric           bool
	absoluteMove     bool
	absoluteArc      bool
	movePlane        int
	completed        bool
	maxArcDeviation  float64
	minArcLineLength float64
	posStack         []Position
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

// Adds a simple linear move
func (vm *Machine) positioning(stmt Statement) {
	newX, newY, newZ, _, _, _ := vm.calcPos(stmt)
	vm.addPos(Position{vm.state, newX, newY, newZ})
}

// Calculates an approximate arc from the provided statement
func (vm *Machine) approximateArc(stmt Statement) {
	var (
		startPos                           Position = vm.curPos()
		endX, endY, endZ, endI, endJ, endK float64  = vm.calcPos(stmt)
		s1, s2, s3, e1, e2, e3, c1, c2     float64
		add                                func(x, y, z float64)
		clockwise                          bool = (vm.state.moveMode == moveModeCWArc)
	)

	vm.state.moveMode = moveModeLinear

	// Read the additional rotation parameter
	P := 0.0
	if pp, ok := stmt['P']; ok {
		P = pp
	}

	//  Flip coordinate system for working in other planes
	switch vm.movePlane {
	case planeXY:
		s1, s2, s3, e1, e2, e3, c1, c2 = startPos.x, startPos.y, startPos.z, endX, endY, endZ, endI, endJ
		add = func(x, y, z float64) {
			vm.positioning(Statement{'X': x, 'Y': y, 'Z': z})
		}
	case planeXZ:
		s1, s2, s3, e1, e2, e3, c1, c2 = startPos.z, startPos.x, startPos.y, endZ, endX, endY, endK, endI
		add = func(x, y, z float64) {
			vm.positioning(Statement{'X': y, 'Y': z, 'Z': x})
		}
	case planeYZ:
		s1, s2, s3, e1, e2, e3, c1, c2 = startPos.y, startPos.z, startPos.x, endY, endZ, endX, endJ, endK
		add = func(x, y, z float64) {
			vm.positioning(Statement{'X': z, 'Y': x, 'Z': y})
		}
	}

	radius1 := math.Sqrt(math.Pow(c1-s1, 2) + math.Pow(c2-s2, 2))
	radius2 := math.Sqrt(math.Pow(c1-e1, 2) + math.Pow(c2-e2, 2))
	if radius1 == 0 || radius2 == 0 {
		panic("Invalid arc statement")
	}

	if math.Abs((radius2-radius1)/radius1) > 0.01 {
		panic(fmt.Sprintf("Radius deviation of %f percent", math.Abs((radius2-radius1)/radius1)*100))
	}

	theta1 := math.Atan2((s2 - c2), (s1 - c1))
	theta2 := math.Atan2((e2 - c2), (e1 - c1))

	angleDiff := theta2 - theta1
	if angleDiff < 0 && !clockwise {
		angleDiff += 2 * math.Pi
	} else if angleDiff > 0 && clockwise {
		angleDiff -= 2 * math.Pi
	}

	if clockwise {
		angleDiff -= P * 2 * math.Pi
	} else {
		angleDiff += P * 2 * math.Pi
	}

	steps := 1
	if vm.maxArcDeviation < radius1 {
		steps = int(math.Ceil(math.Abs(angleDiff / (2 * math.Acos(1-vm.maxArcDeviation/radius1)))))
	}

	// Enforce a minimum line length
	arcLen := math.Abs(angleDiff) * math.Sqrt(math.Pow(radius1, 2)+math.Pow((e3-s3)/angleDiff, 2))
	steps2 := int(arcLen / vm.minArcLineLength)

	if steps > steps2 {
		steps = steps2
	}

	angle := 0.0
	for i := 0; i <= steps; i++ {
		angle = theta1 + angleDiff/float64(steps)*float64(i)
		a1, a2 := c1+radius1*math.Cos(angle), c2+radius1*math.Sin(angle)
		a3 := s3 + (e3-s3)/float64(steps)*float64(i)
		add(a1, a2, a3)
	}
	add(e1, e2, e3)
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
			vm.approximateArc(stmt)
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

// Process AST
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

// Initialize the VM
func (vm *Machine) Init(maxArcDeviation, minArcLineLength float64) {
	vm.posStack = append(vm.posStack, Position{})
	vm.metric = true
	vm.absoluteMove = true
	vm.absoluteArc = false
	vm.movePlane = planeXY
	vm.maxArcDeviation = maxArcDeviation
	vm.minArcLineLength = minArcLineLength
}
