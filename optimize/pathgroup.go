package optimize

import "github.com/kennylevinsen/gocnc/vm"
import "github.com/kennylevinsen/gocnc/vector"

import "errors"
import "fmt"

// Reduces moves between paths.
// It does this by scanning through position stack, grouping moves that move from >= Z0 to < Z0.
// These moves are then sorted after closest to previous position, starting at X0 Y0,
// and moves to groups recalculated as they are inserted in a new stack.
// This optimization pass bails if the Z axis is moved simultaneously with any other axis,
// or the input ends with the drill below Z0, in order to play it safe.
// This pass is new, and therefore slightly experimental.
func OptPathGrouping(machine *vm.Machine, tolerance float64) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New(fmt.Sprintf("%s", r))
		}
	}()

	type Set []vm.Position
	var (
		lastx, lasty, lastz float64
		sets                []Set = make([]Set, 0)
		curSet              Set   = make(Set, 0)
		safetyHeight        float64
		drillSpeed          float64
		sequenceStarted     bool = false
	)

	// Find grouped drills
	for _, m := range machine.Positions {
		if m.Z != lastz && (m.X != lastx || m.Y != lasty) {
			panic("Complex z-motion detected")
		}

		if m.X == lastx && m.Y == lasty {
			if lastz >= 0 && m.Z < 0 {
				// Down move
				sequenceStarted = true

				// Set drill feedrate
				if m.State.MoveMode == vm.MoveModeLinear && m.State.Feedrate > drillSpeed {
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
				sequenceStarted = false
				curSet = make(Set, 0)
				goto updateLast // Skip append
			}

		} else {
			if m.Z < 0 && m.State.MoveMode == vm.MoveModeRapid {
				panic("Rapid move in stock detected")
			}
		}

		if sequenceStarted {
			// Regular move
			if m.Z > 0 {
				panic("Move above stock detected")
			}
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
		curVec      vector.Vector
		sortedSets  []Set = make([]Set, 0)
		selectedSet int
	)

	// Stupid difference calculator
	xyDiff := func(pos vector.Vector, cur vector.Vector) float64 {
		j := cur.Diff(pos)
		j.Z = 0
		return j.Norm()
	}

	// Sort the sets after distance from current position
	for len(sets) > 0 {
		for idx := range sets {
			if selectedSet == -1 {
				selectedSet = idx
			} else {
				np := sets[idx][0]
				pp := sets[selectedSet][0]
				diff := xyDiff(np.Vector(), curVec)
				other := xyDiff(pp.Vector(), curVec)
				if diff < other {
					selectedSet = idx
				} else if np.Z > pp.Z {
					selectedSet = idx
				}
			}
		}
		curVec = sets[selectedSet][0].Vector()
		sortedSets = append(sortedSets, sets[selectedSet])
		sets = append(sets[0:selectedSet], sets[selectedSet+1:]...)
		selectedSet = -1
	}

	// Reconstruct new position stack from sorted sections
	newPos := []vm.Position{machine.Positions[0]} // Origin

	addPos := func(pos vm.Position) {
		newPos = append(newPos, pos)
	}

	moveTo := func(pos vm.Position) {
		curPos := newPos[len(newPos)-1]

		// Check if we should go to safety-height before moving
		if xyDiff(curPos.Vector(), pos.Vector()) < tolerance {
			if curPos.X != pos.X || curPos.Y != pos.Y {
				// If we're not 100% precise...
				step1 := curPos
				step1.State.MoveMode = vm.MoveModeLinear
				step1.X = pos.X
				step1.Y = pos.Y
				addPos(step1)
			}
			addPos(pos)
		} else {
			step1 := curPos
			step1.Z = safetyHeight
			step1.State.MoveMode = vm.MoveModeRapid
			step2 := step1
			step2.X, step2.Y = pos.X, pos.Y
			step3 := step2
			step3.Z = pos.Z
			step3.State.MoveMode = vm.MoveModeLinear
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

	machine.Positions = newPos

	return nil
}
