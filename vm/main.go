package vm

import "github.com/joushou/gocnc/gcode"
import "fmt"

type Statement map[rune]float64

//
// State structs
//

const (
	rapidMoveMode  = iota
	linearMoveMode = iota
	cwArcMode      = iota
	ccwArcMode     = iota
)

type State struct {
	feedrate       float64
	spindleEnabled bool
}

type Position struct {
	x, y, z float64
	state   State
}

type Move struct {
	prevPos  Position
	newPos   Position
	moveMode int
	i, j, k  float64
}

type Machine struct {
	curPos    Position
	mode      string
	moveStack []Move
}

//
// Positioning
//

func (vm *Machine) rapidPos(stmt Statement) {
	var newX, newY, newZ float64
	var ok bool
	if newX, ok = stmt['X']; !ok {
		newX = vm.curPos.x
	}
	if newY, ok = stmt['Y']; !ok {
		newY = vm.curPos.y
	}
	if newZ, ok = stmt['Z']; !ok {
		newZ = vm.curPos.z
	}
	newPos := Position{newX, newY, newZ, vm.curPos.state}
	vm.moveStack = append(vm.moveStack, Move{vm.curPos, newPos, rapidMoveMode, 0, 0, 0})
	vm.curPos = newPos
}

func (vm *Machine) linearPos(stmt Statement) {
	var newX, newY, newZ float64
	var ok bool
	if newX, ok = stmt['X']; !ok {
		newX = vm.curPos.x
	}
	if newY, ok = stmt['Y']; !ok {
		newY = vm.curPos.y
	}
	if newZ, ok = stmt['Z']; !ok {
		newZ = vm.curPos.z
	}
	newPos := Position{newX, newY, newZ, vm.curPos.state}
	vm.moveStack = append(vm.moveStack, Move{vm.curPos, newPos, linearMoveMode, 0, 0, 0})
	vm.curPos = newPos
}

func (vm *Machine) cwArc(stmt Statement) {
	var newX, newY, newZ, newI, newJ, newK float64
	var ok bool
	if newX, ok = stmt['X']; !ok {
		newX = vm.curPos.x
	}
	if newY, ok = stmt['Y']; !ok {
		newY = vm.curPos.y
	}
	if newZ, ok = stmt['Z']; !ok {
		newZ = vm.curPos.z
	}
	if newI, ok = stmt['I']; !ok {
		newI = 0
	}
	if newJ, ok = stmt['J']; !ok {
		newJ = 0
	}
	if newK, ok = stmt['K']; !ok {
		newK = 0
	}
	newPos := Position{newX, newY, newZ, vm.curPos.state}
	vm.moveStack = append(vm.moveStack, Move{vm.curPos, newPos, cwArcMode, newI, newJ, newK})
	vm.curPos = newPos
}

func (vm *Machine) ccwArc(stmt Statement) {
	var newX, newY, newZ, newI, newJ, newK float64
	var ok bool
	if newX, ok = stmt['X']; !ok {
		newX = vm.curPos.x
	}
	if newY, ok = stmt['Y']; !ok {
		newY = vm.curPos.y
	}
	if newZ, ok = stmt['Z']; !ok {
		newZ = vm.curPos.z
	}
	if newI, ok = stmt['I']; !ok {
		newI = 0
	}
	if newJ, ok = stmt['J']; !ok {
		newJ = 0
	}
	if newK, ok = stmt['K']; !ok {
		newK = 0
	}
	newPos := Position{newX, newY, newZ, vm.curPos.state}
	vm.moveStack = append(vm.moveStack, Move{vm.curPos, newPos, ccwArcMode, newI, newJ, newK})
	vm.curPos = newPos
}

//
// Dispatch
//

func (vm *Machine) run(stmt Statement) {
	if g, ok := stmt['G']; ok {
		switch g {
		case 0:
			vm.mode = "rapid-pos"
		case 1:
			vm.mode = "linear-pos"
		case 2:
			vm.mode = "cw-arc"
		case 3:
			vm.mode = "ccw-arc"
		}
	} else if g, ok := stmt['F']; ok {
		vm.curPos.state.feedrate = g
	}

	switch vm.mode {
	case "rapid-pos":
		vm.rapidPos(stmt)
	case "linear-pos":
		vm.linearPos(stmt)
	case "cw-arc":
		vm.cwArc(stmt)
	case "ccw-arc":
		vm.ccwArc(stmt)
	}
}

func (vm *Machine) Dump() {
	for _, m := range vm.moveStack {
		switch m.moveMode {
		case rapidMoveMode:
			fmt.Printf("rapid move, ")
		case linearMoveMode:
			fmt.Printf("linear move, ")
		case cwArcMode:
			fmt.Printf("clockwise arc, ")
		case ccwArcMode:
			fmt.Printf("counterclockwise arc, ")
		}

		fmt.Printf("feedrate: %f, ", m.newPos.state.feedrate)
		fmt.Printf("X: %f, Y: %f, Z: %f\n", m.newPos.x, m.newPos.y, m.newPos.z)
	}
}

func (vm *Machine) Process(doc *gcode.Document) {
	stmt := make(Statement)
	for _, b := range doc.Blocks {
		if b.BlockDelete {
			continue
		}

		for _, n := range b.Nodes {
			if word, ok := n.(*gcode.Word); ok {
				stmt[word.Address] = word.Command
			}
		}
		vm.run(stmt)
	}
}
