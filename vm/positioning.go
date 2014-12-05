package vm

import "github.com/joushou/gocnc/gcode"
import "github.com/joushou/gocnc/vector"
import "math"

import "fmt"

// Retrieves position from top of stack
func (vm *Machine) curPos() Position {
	return vm.Positions[len(vm.Positions)-1]
}

// Appends a position to the stack
func (vm *Machine) move(newX, newY, newZ float64) {
	pos := Position{vm.State, newX, newY, newZ}
	vm.Positions = append(vm.Positions, pos)
}

// Calculates the absolute position of the given statement, including optional I, J, K parameters
func (vm *Machine) calcPos(stmt gcode.Block) (newX, newY, newZ, newI, newJ, newK float64) {
	pos := vm.curPos()
	var err error

	if newX, err = stmt.GetWord('X'); err != nil {
		newX = pos.X
	} else if vm.Imperial {
		newX *= 25.4
	}

	if newY, err = stmt.GetWord('Y'); err != nil {
		newY = pos.Y
	} else if vm.Imperial {
		newY *= 25.4
	}

	if newZ, err = stmt.GetWord('Z'); err != nil {
		newZ = pos.Z
	} else if vm.Imperial {
		newZ *= 25.4
	}

	newI = stmt.GetWordDefault('I', 0.0)
	newJ = stmt.GetWordDefault('J', 0.0)
	newK = stmt.GetWordDefault('K', 0.0)

	if vm.Imperial {
		newI *= 25.4
		newJ *= 25.4
		newK *= 25.4
	}

	if !vm.AbsoluteMove {
		newX += pos.X
		newY += pos.Y
		newZ += pos.Z
	}

	if !vm.AbsoluteArc {
		newI += pos.X
		newJ += pos.Y
		newK += pos.Z
	}

	return newX, newY, newZ, newI, newJ, newK
}

// Calculates an approximate arc from the provided statement
func (vm *Machine) arc(endX, endY, endZ, endI, endJ, endK, P float64) {
	var (
		startPos                       Position = vm.curPos()
		s1, s2, s3, e1, e2, e3, c1, c2 float64
		add                            func(x, y, z float64)
		clockwise                      bool = (vm.State.MoveMode == MoveModeCWArc)
	)

	oldState := vm.State.MoveMode
	vm.State.MoveMode = MoveModeLinear

	//  Flip coordinate system for working in other planes
	switch vm.MovePlane {
	case PlaneXY:
		s1, s2, s3, e1, e2, e3, c1, c2 = startPos.X, startPos.Y, startPos.Z, endX, endY, endZ, endI, endJ
		add = func(x, y, z float64) {
			vm.move(x, y, z)
		}
	case PlaneXZ:
		s1, s2, s3, e1, e2, e3, c1, c2 = startPos.Z, startPos.X, startPos.Y, endZ, endX, endY, endK, endI
		add = func(x, y, z float64) {
			vm.move(y, z, x)
		}
	case PlaneYZ:
		s1, s2, s3, e1, e2, e3, c1, c2 = startPos.Y, startPos.Z, startPos.X, endY, endZ, endX, endJ, endK
		add = func(x, y, z float64) {
			vm.move(z, x, y)
		}
	}

	radius1 := math.Sqrt(math.Pow(c1-s1, 2) + math.Pow(c2-s2, 2))
	radius2 := math.Sqrt(math.Pow(c1-e1, 2) + math.Pow(c2-e2, 2))
	if radius1 == 0 || radius2 == 0 {
		panic("Invalid arc statement")
	}

	if deviation := math.Abs((radius2-radius1)/radius1) * 100; deviation > 0.6 {
		e := vector.Vector{e1, e2, e3}
		m := vector.Vector{e1 - math.Abs(c1-s1), e2 - math.Abs(c2-s2), e3}
		x := e.Diff(m).Norm()
		if x > 0.1 {
			panic(fmt.Sprintf("Radius deviation of %f percent and %f mm", deviation, x))
		}
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
	if vm.MaxArcDeviation < radius1 {
		steps = int(math.Ceil(math.Abs(angleDiff / (2 * math.Acos(1-vm.MaxArcDeviation/radius1)))))
	}

	// Enforce a minimum line length
	arcLen := math.Abs(angleDiff) * math.Sqrt(math.Pow(radius1, 2)+math.Pow((e3-s3)/angleDiff, 2))
	steps2 := int(arcLen / vm.MinArcLineLength)

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

	vm.State.MoveMode = oldState
}
