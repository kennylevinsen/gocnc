package vm

import "errors"
import "fmt"

// Limit feedrate.
func (vm *Machine) LimitFeedrate(feed float64) {
	for idx, m := range vm.Positions {
		if m.State.Feedrate > feed {
			vm.Positions[idx].State.Feedrate = feed
		}
	}
}

// Increase feedrate
func (vm *Machine) FeedrateMultiplier(feedMultiplier float64) {
	for idx, _ := range vm.Positions {
		vm.Positions[idx].State.Feedrate *= feedMultiplier
	}
}

// Enforce spindle mode
func (vm *Machine) EnforceSpindle(enabled, clockwise bool, speed float64) {
	for idx, _ := range vm.Positions {
		vm.Positions[idx].State.SpindleSpeed = speed
		vm.Positions[idx].State.SpindleEnabled = enabled
		vm.Positions[idx].State.SpindleClockwise = clockwise
	}
}

// Set safety-height.
// Scans for the highest position on the Y axis, and afterwards replaces all instances
// of this position with the requested height.
func (vm *Machine) SetSafetyHeight(height float64) error {
	// Ensure we detected the highest point in the script - we don't want any collisions

	var maxz, nextz float64
	for _, m := range vm.Positions {
		if m.Z > maxz {
			nextz = maxz
			maxz = m.Z
		}
		if m.Z > nextz && m.Z < maxz {
			nextz = m.Z
		}
	}

	if height <= nextz {
		return errors.New(fmt.Sprintf("New safety height collides with lower feed height of %g", nextz))
	}

	// Apply the changes
	var lastx, lasty float64
	for idx, m := range vm.Positions {
		if lastx == m.X && lasty == m.Y && m.Z == maxz {
			vm.Positions[idx].Z = height
		}
		lastx, lasty = m.X, m.Y
	}
	return nil
}

// Ensure return to X0 Y0 Z0.
// Simply adds a what is necessary to move back to X0 Y0 Z0.
func (vm *Machine) Return() {
	var maxz float64
	for _, m := range vm.Positions {
		if m.Z > maxz {
			maxz = m.Z
		}
	}

	lastPos := vm.Positions[len(vm.Positions)-1]
	if lastPos.X == 0 && lastPos.Y == 0 && lastPos.Z == 0 {
		return
	} else if lastPos.X == 0 && lastPos.Y == 0 && lastPos.Z != 0 {
		lastPos.Z = 0
		lastPos.State.MoveMode = MoveModeRapid
		vm.Positions = append(vm.Positions, lastPos)
		return
	} else if lastPos.Z == maxz {
		move1 := lastPos
		move1.X = 0
		move1.Y = 0
		move1.State.MoveMode = MoveModeRapid
		move2 := move1
		move2.Z = 0
		vm.Positions = append(vm.Positions, move1)
		vm.Positions = append(vm.Positions, move2)
		return
	} else {
		move1 := lastPos
		move1.Z = maxz
		move1.State.MoveMode = MoveModeRapid
		move2 := move1
		move2.X = 0
		move2.Y = 0
		move3 := move2
		move3.Z = 0
		vm.Positions = append(vm.Positions, move1)
		vm.Positions = append(vm.Positions, move2)
		vm.Positions = append(vm.Positions, move3)
		return
	}
}

// Generate move information
func (vm *Machine) Info() (minx, miny, minz, maxx, maxy, maxz float64, feedrates []float64) {
	for _, pos := range vm.Positions {
		if pos.X < minx {
			minx = pos.X
		} else if pos.X > maxx {
			maxx = pos.X
		}

		if pos.Y < miny {
			miny = pos.Y
		} else if pos.Y > maxy {
			maxy = pos.Y
		}

		if pos.Z < minz {
			minz = pos.Z
		} else if pos.Z > maxz {
			maxz = pos.Z
		}

		feedrateFound := false
		for _, feed := range feedrates {
			if feed == pos.State.Feedrate {
				feedrateFound = true
				break
			}
		}
		if !feedrateFound {
			feedrates = append(feedrates, pos.State.Feedrate)
		}
	}
	return
}
