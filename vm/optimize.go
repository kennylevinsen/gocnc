package vm

import "math"
import "errors"
import "strconv"
import "fmt"

//
// Ideas for other optimization steps:
//   Move grouping - Group moves based on Z0, Zdepth lifts, to finalize
//      section, instead of constantly moving back and forth
//   Vector-angle removal - Combine moves where the move vector changes
//      less than a certain minimum angle
//

//
// Detects a previous drill, and uses rapid move to the previous known depth
//
func (vm *Machine) OptDrillSpeed() {
	var (
		lastx, lasty, lastz float64
		npos                []Position = make([]Position, 0)
		drillStack          []Position = make([]Position, 0)
	)

	fastDrill := func(pos Position) (Position, Position, bool) {
		var depth float64
		var found bool
		for _, m := range drillStack {
			if m.x == pos.x && m.y == pos.y {
				if m.z < depth {
					depth = m.z
					found = true
				}
			}
		}

		drillStack = append(drillStack, pos)

		if found {
			if pos.z >= depth { // We have drilled all of it, so just rapid all the way
				pos.state.moveMode = moveModeRapid
				return pos, pos, false
			} else { // Can only rapid some of the way
				p := pos
				p.z = depth
				p.state.moveMode = moveModeRapid
				return p, pos, true
			}
		} else {
			return pos, pos, false
		}
	}

	for _, m := range vm.posStack {
		if m.x == lastx && m.y == lasty && m.z < lastz && m.state.moveMode == moveModeLinear {
			posn, poso, shouldinsert := fastDrill(m)
			if shouldinsert {
				npos = append(npos, posn)
			}
			npos = append(npos, poso)
		} else {
			npos = append(npos, m)
		}
		lastx, lasty, lastz = m.x, m.y, m.z
	}
	vm.posStack = npos
}

//
// Reduces moves between routing operations
//
func (vm *Machine) OptRouteGrouping() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New(fmt.Sprintf("%s", r))
		}
	}()

	type Set []Position
	var (
		lastx, lasty, lastz float64
		sets                []Set = make([]Set, 0)
		curSet              Set   = make(Set, 0)
		safetyHeight        float64
	)

	// Find grouped drills
	for _, m := range vm.posStack {
		if m.z != lastz && (m.x != lastx || m.y != lasty) {
			panic("Complex z-motion detected")
		}

		if m.x == lastx && m.y == lasty {
			if lastz >= 0 && m.z < 0 {
				// Down move
				curSet = append(curSet, m)
			} else if lastz < 0 && m.z >= 0 {
				// Up move - ignored in set
				//curSet = append(curSet, m)
				sets = append(sets, curSet)
				curSet = make(Set, 0)
			} else {
				// Minor z-move
				curSet = append(curSet, m)
			}
		} else {
			// Regular move
			curSet = append(curSet, m)
		}

		if m.z > safetyHeight {
			safetyHeight = m.z
		}
		lastx, lasty, lastz = m.x, m.y, m.z
	}

	// If there was a final set without a proper lift
	if len(curSet) == 1 {
		p := curSet[0]
		if p.z != safetyHeight || lastz != safetyHeight || p.x != 0 || p.y != 0 {
			panic("Incomplete final drill set")
		}
	} else if len(curSet) > 0 {
		panic("Incomplete final drill set")
	}

	var (
		curX, curY, curZ float64 = 0, 0, 0
		sortedSets       []Set   = make([]Set, 0)
		selectedSet      int
	)

	// Stupid difference calculator
	diffFromCur := func(pos Position) float64 {
		x := math.Max(curX, pos.x) - math.Min(curX, pos.x)
		y := math.Max(curY, pos.y) - math.Min(curY, pos.y)
		z := math.Max(curZ, pos.z) - math.Min(curZ, pos.z)
		return math.Sqrt(math.Pow(x, 2) + math.Pow(y, 2) + math.Pow(z, 2))
	}

	// Sort the sets after distance from current position
	for len(sets) > 0 {
		for idx, _ := range sets {
			if selectedSet == -1 {
				selectedSet = idx
			} else {
				diff := diffFromCur(sets[idx][0])
				other := diffFromCur(sets[selectedSet][0])
				if diff < other {
					selectedSet = idx
				}
			}
		}
		curX, curY, curZ = sets[selectedSet][0].x, sets[selectedSet][0].y, sets[selectedSet][0].z
		sortedSets = append(sortedSets, sets[selectedSet])
		sets = append(sets[0:selectedSet], sets[selectedSet+1:]...)
		selectedSet = -1
	}

	// Reconstruct new position stack from sorted sections
	newPos := make([]Position, 0)
	newPos = append(newPos, vm.posStack[0]) // The first null-move

	addPos := func(pos Position) {
		newPos = append(newPos, pos)
	}

	moveTo := func(pos Position) {
		curPos := newPos[len(newPos)-1]
		if curPos.x == pos.x && curPos.y == pos.y {
			if pos.z == safetyHeight {
				// Redundant lift
				return
			} else {
				addPos(pos)
			}
		} else {
			step1 := newPos[len(newPos)-1]
			step1.z = safetyHeight
			step2 := step1
			step2.x, step2.y = pos.x, pos.y
			step3 := step2
			step3.z = pos.z

			addPos(step1)
			addPos(step2)
			addPos(step3)
		}

	}

	for _, m := range sortedSets {
		for idx, p := range m {
			if idx == 0 {
				moveTo(p)
			} else {
				addPos(p)
			}
		}
	}

	vm.posStack = newPos

	return nil
}

//
// Uses rapid move for all Z-up only moves
//
func (vm *Machine) OptLiftSpeed() {
	var lastx, lasty, lastz float64
	for idx, m := range vm.posStack {
		if m.x == lastx && m.y == lasty && m.z > lastz {
			// We got a lift! Let's make it faster, shall we?
			vm.posStack[idx].state.moveMode = moveModeRapid
		}
		lastx, lasty, lastz = m.x, m.y, m.z
	}
}

//
// Kills redundant partial moves
//
func (vm *Machine) OptBogusMoves() {
	var (
		xstate, ystate, zstate       float64
		vecX, vecY, vecZ             float64
		lastvecX, lastvecY, lastvecZ float64
		npos                         []Position = make([]Position, 0)
	)

	for _, m := range vm.posStack {
		dx, dy, dz := m.x-xstate, m.y-ystate, m.z-zstate
		xstate, ystate, zstate = m.x, m.y, m.z

		if m.state.moveMode != moveModeRapid && m.state.moveMode != moveModeLinear {
			// I'm not mentally ready for arc optimization yet...
			npos = append(npos, m)
			continue
		}

		if dx == 0 && dz == 0 && dy == 0 {
			// Why are we doing this again?!
			continue
		}

		norm := math.Sqrt(math.Pow(dx, 2) + math.Pow(dy, 2) + math.Pow(dz, 2))
		vecX, vecY, vecZ = dx/norm, dy/norm, dz/norm

		if lastvecX == vecX && lastvecY == vecY && lastvecZ == vecZ {
			npos[len(npos)-1] = m
		} else {
			npos = append(npos, m)
			lastvecX, lastvecY, lastvecZ = vecX, vecY, vecZ
		}
	}
	vm.posStack = npos
}

//
// Limit feedrate
//
func (vm *Machine) LimitFeedrate(feed float64) {
	for idx, m := range vm.posStack {
		if m.state.feedrate > feed {
			vm.posStack[idx].state.feedrate = feed
		}
	}
}

//
// Set safety-height
//
func (vm *Machine) SetSafetyHeight(height float64) error {
	// Ensure we detected the highest point in the script - we don't want any collisions

	var maxz, nextz float64
	for _, m := range vm.posStack {
		if m.z > maxz {
			nextz = maxz
			maxz = m.z
		}
		if m.z > nextz && m.z < maxz {
			nextz = m.z
		}
	}

	if height <= nextz {
		return errors.New("New safety height collides with lower feed height of " + strconv.FormatFloat(nextz, 'f', -1, 64))
	}

	// Apply the changes
	var lastx, lasty float64
	for idx, m := range vm.posStack {
		if lastx == m.x && lasty == m.y && m.z == maxz {
			vm.posStack[idx].z = height
		}
		lastx, lasty = m.x, m.y
	}
	return nil
}

//
// Ensure return to X0 Y0 Z0
//
func (vm *Machine) Return() {
	var maxz float64
	for _, m := range vm.posStack {
		if m.z > maxz {
			maxz = m.z
		}
	}

	lastPos := vm.posStack[len(vm.posStack)-1]
	if lastPos.x == 0 && lastPos.y == 0 && lastPos.z == 0 {
		return
	} else if lastPos.x == 0 && lastPos.y == 0 && lastPos.z != 0 {
		lastPos.z = 0
		lastPos.state.moveMode = moveModeRapid
		vm.posStack = append(vm.posStack, lastPos)
		return
	} else if lastPos.z == maxz {
		move1 := lastPos
		move1.x = 0
		move1.y = 0
		move1.state.moveMode = moveModeRapid
		move2 := move1
		move2.z = 0
		vm.posStack = append(vm.posStack, move1)
		vm.posStack = append(vm.posStack, move2)
		return
	} else {
		move1 := lastPos
		move1.z = maxz
		move1.state.moveMode = moveModeRapid
		move2 := move1
		move2.x = 0
		move2.y = 0
		move3 := move2
		move3.z = 0
		vm.posStack = append(vm.posStack, move1)
		vm.posStack = append(vm.posStack, move2)
		vm.posStack = append(vm.posStack, move3)
		return
	}
}
