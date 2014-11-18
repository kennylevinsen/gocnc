package vm

import "github.com/joushou/gocnc/utils"

import "math"
import "errors"
import "fmt"

//
// Ideas for other optimization steps:
//   Move grouping - Group moves based on Z0, Zdepth lifts, to finalize
//      section, instead of constantly moving back and forth
//   Vector-angle removal - Combine moves where the move vector changes
//      less than a certain minimum angle
//

// Detects a previous drill, and uses rapid move to the previous known depth.
// Scans through all Z-descent moves, logs its height, and ensures that any future move
// at that location will use MoveModeRapid to go to the deepest previous known Z-height.
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
			if m.X == pos.X && m.Y == pos.Y {
				if m.Z < depth {
					depth = m.Z
					found = true
				}
			}
		}

		drillStack = append(drillStack, pos)

		if found {
			if pos.Z >= depth { // We have drilled all of it, so just rapid all the way
				pos.State.MoveMode = MoveModeRapid
				return pos, pos, false
			} else { // Can only rapid some of the way
				p := pos
				p.Z = depth
				p.State.MoveMode = MoveModeRapid
				return p, pos, true
			}
		} else {
			return pos, pos, false
		}
	}

	for _, m := range vm.Positions {
		if m.X == lastx && m.Y == lasty && m.Z < lastz && m.State.MoveMode == MoveModeLinear {
			posn, poso, shouldinsert := fastDrill(m)
			if shouldinsert {
				npos = append(npos, posn)
			}
			npos = append(npos, poso)
		} else {
			npos = append(npos, m)
		}
		lastx, lasty, lastz = m.X, m.Y, m.Z
	}
	vm.Positions = npos
}

// Reduces moves between routing operations.
// It does this by scanning through position stack, grouping moves that move from >= Z0 to < Z0.
// These moves are then sorted after closest to previous position, starting at X0 Y0,
// and moves to groups recalculated as they are inserted in a new stack.
// This optimization pass bails if the Z axis is moved simultaneously with any other axis,
// or the input ends with the drill below Z0, in order to play it safe.
// This pass is new, and therefore slightly experimental.
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
		drillSpeed          float64
		sequenceStarted     bool = false
	)

	// Find grouped drills
	for _, m := range vm.Positions {
		if m.Z != lastz && (m.X != lastx || m.Y != lasty) {
			panic("Complex z-motion detected")
		}

		if m.X == lastx && m.Y == lasty {
			if lastz >= 0 && m.Z < 0 {
				// Down move
				sequenceStarted = true
				curSet = append(curSet, m)

				// Set drill feedrate
				if m.State.MoveMode == MoveModeLinear && m.State.Feedrate > drillSpeed {
					if drillSpeed != 0 {
						panic("Multiple drill feedrates detected")
					}
					drillSpeed = m.State.Feedrate
				}
			} else if lastz < 0 && m.Z >= 0 {
				// Up move - ignored in set
				//curSet = append(curSet, m)
				if sequenceStarted {
					sets = append(sets, curSet)
				}
				curSet = make(Set, 0)
				goto updateLast // Skip append
			}
		}
		if sequenceStarted {
			// Regular move
			curSet = append(curSet, m)
		}

	updateLast:
		if m.Z > safetyHeight {
			safetyHeight = m.Z
		}
		lastx, lasty, lastz = m.X, m.Y, m.Z
	}

	if safetyHeight == 0 {
		panic("Unable to detect safety height")
	} else if drillSpeed == 0 {
		panic("Unable to detect drill feedrate")
	}

	// If there was a final set without a proper lift
	if len(curSet) == 1 {
		p := curSet[0]
		if p.Z != safetyHeight || lastz != safetyHeight || p.X != 0 || p.Y != 0 {
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
		x := math.Max(curX, pos.X) - math.Min(curX, pos.X)
		y := math.Max(curY, pos.Y) - math.Min(curY, pos.Y)
		z := math.Max(curZ, pos.Z) - math.Min(curZ, pos.Z)
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
		curX, curY, curZ = sets[selectedSet][0].X, sets[selectedSet][0].Y, sets[selectedSet][0].Z
		sortedSets = append(sortedSets, sets[selectedSet])
		sets = append(sets[0:selectedSet], sets[selectedSet+1:]...)
		selectedSet = -1
	}

	// Reconstruct new position stack from sorted sections
	newPos := make([]Position, 0)
	newPos = append(newPos, vm.Positions[0]) // The first null-move

	addPos := func(pos Position) {
		newPos = append(newPos, pos)
	}

	moveTo := func(pos Position) {
		curPos := newPos[len(newPos)-1]

		// Check if we should go to safety-height before moving
		if math.Abs(curPos.X-pos.X) < vm.Tolerance && math.Abs(curPos.Y-pos.Y) < vm.Tolerance {
			if curPos.X != pos.X || curPos.Y != pos.Y {
				// If we're not 100% precise...
				step1 := curPos
				step1.State.MoveMode = MoveModeLinear
				step1.X = pos.X
				step1.Y = pos.Y
				addPos(step1)
			}
			if pos.Z == safetyHeight {
				// Redundant lift
				return
			} else {
				addPos(pos)
			}
		} else {
			step1 := curPos
			step1.Z = safetyHeight
			step1.State.MoveMode = MoveModeRapid
			step2 := step1
			step2.X, step2.Y = pos.X, pos.Y
			step3 := step2
			step3.Z = pos.Z
			step3.State.MoveMode = MoveModeLinear
			step3.State.Feedrate = drillSpeed

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

	vm.Positions = newPos

	return nil
}

// Uses rapid move for all Z-up only moves.
// Scans all positions for moves that only change the z-axis in a positive direction,
// and sets the moveMode to MoveModeRapid.
func (vm *Machine) OptLiftSpeed() {
	var lastx, lasty, lastz float64
	for idx, m := range vm.Positions {
		if m.X == lastx && m.Y == lasty && m.Z > lastz {
			// We got a lift! Let's make it faster, shall we?
			vm.Positions[idx].State.MoveMode = MoveModeRapid
		}
		lastx, lasty, lastz = m.X, m.Y, m.Z
	}
}

// Kills redundant partial moves.
// Calculates the unit-vector, and kills all incremental moves between A and B.
func (vm *Machine) OptBogusMoves() {
	var (
		xstate, ystate, zstate       float64
		vecX, vecY, vecZ             float64
		lastvecX, lastvecY, lastvecZ float64
		npos                         []Position = make([]Position, 0)
	)

	for _, m := range vm.Positions {
		dx, dy, dz := m.X-xstate, m.Y-ystate, m.Z-zstate
		xstate, ystate, zstate = m.X, m.Y, m.Z

		if m.State.MoveMode != MoveModeRapid && m.State.MoveMode != MoveModeLinear {
			lastvecX, lastvecY, lastvecZ = 0, 0, 0
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
	vm.Positions = npos
}

// Kills redundant partial moves.
// Calculates the unit-vector, and kills all incremental moves between A and B.
func (vm *Machine) OptVector() {
	var (
		vec1, vec2, vec3, u, ca, cb utils.Vector
		ready                       int
		dist, angle1, angle2        float64
		npos                        []Position = make([]Position, 0)
	)

	for _, m := range vm.Positions {
		if m.State.MoveMode != MoveModeRapid && m.State.MoveMode != MoveModeLinear {
			ready = 0
			goto appendpos
		}

		if ready == 0 {
			vec1 = utils.Vector{m.X, m.Y, m.Z}
			ready++
			goto appendpos
		} else if ready == 1 {
			vec2 = utils.Vector{m.X, m.Y, m.Z}
			ready++
			goto appendpos
		} else if ready == 2 {
			vec3 = utils.Vector{m.X, m.Y, m.Z}
			ready++
		} else {
			vec1 = vec2
			vec2 = vec3
			vec3 = utils.Vector{m.X, m.Y, m.Z}
		}

		u = vec1.Diff(vec3).Divide(vec1.Diff(vec3).Norm())

		ca = vec2.Diff(vec1)
		cb = vec2.Diff(vec3)

		angle1 = ca.Dot(u) / (ca.Norm() * u.Norm())
		angle2 = cb.Dot(u) / (cb.Norm() * u.Norm())
		if angle1 > 0 || angle2 < 0 {
			fmt.Printf("Vectors:\n")
			fmt.Printf("%f, %f, %f\n", vec1.X, vec1.Y, vec1.Z)
			fmt.Printf("%f, %f, %f\n", vec2.X, vec2.Y, vec2.Z)
			fmt.Printf("%f, %f, %f\n", vec3.X, vec3.Y, vec3.Z)
		}
		dist = ca.Cross(u).Norm() / u.Norm()

		if dist < vm.Tolerance {
			npos[len(npos)-1] = m
			vec2 = vec1
			continue
		}

	appendpos:
		npos = append(npos, m)
	}
	vm.Positions = npos
}
