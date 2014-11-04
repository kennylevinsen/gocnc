package vm

import "errors"
import "fmt"

// Limit feedrate.
func (vm *Machine) LimitFeedrate(feed float64) {
	for idx, m := range vm.Positions {
		if m.state.feedrate > feed {
			vm.Positions[idx].state.feedrate = feed
		}
	}
}

// Increase feedrate
func (vm *Machine) FeedrateMultiplier(feedMultiplier float64) {
	for idx, _ := range vm.Positions {
		vm.Positions[idx].state.feedrate *= feedMultiplier
	}
}

// Enforce spindle mode
func (vm *Machine) EnforceSpindle(enabled, clockwise bool, speed float64) {
	for idx, _ := range vm.Positions {
		vm.Positions[idx].state.spindleSpeed = speed
		vm.Positions[idx].state.spindleEnabled = enabled
		vm.Positions[idx].state.spindleClockwise = clockwise
	}
}

// Set safety-height.
// Scans for the highest position on the Y axis, and afterwards replaces all instances
// of this position with the requested height.
func (vm *Machine) SetSafetyHeight(height float64) error {
	// Ensure we detected the highest point in the script - we don't want any collisions

	var maxz, nextz float64
	for _, m := range vm.Positions {
		if m.z > maxz {
			nextz = maxz
			maxz = m.z
		}
		if m.z > nextz && m.z < maxz {
			nextz = m.z
		}
	}

	if height <= nextz {
		return errors.New(fmt.Sprintf("New safety height collides with lower feed height of %g", nextz))
	}

	// Apply the changes
	var lastx, lasty float64
	for idx, m := range vm.Positions {
		if lastx == m.x && lasty == m.y && m.z == maxz {
			vm.Positions[idx].z = height
		}
		lastx, lasty = m.x, m.y
	}
	return nil
}

// Ensure return to X0 Y0 Z0.
// Simply adds a what is necessary to move back to X0 Y0 Z0.
func (vm *Machine) Return() {
	var maxz float64
	for _, m := range vm.Positions {
		if m.z > maxz {
			maxz = m.z
		}
	}

	lastPos := vm.Positions[len(vm.Positions)-1]
	if lastPos.x == 0 && lastPos.y == 0 && lastPos.z == 0 {
		return
	} else if lastPos.x == 0 && lastPos.y == 0 && lastPos.z != 0 {
		lastPos.z = 0
		lastPos.state.moveMode = moveModeRapid
		vm.Positions = append(vm.Positions, lastPos)
		return
	} else if lastPos.z == maxz {
		move1 := lastPos
		move1.x = 0
		move1.y = 0
		move1.state.moveMode = moveModeRapid
		move2 := move1
		move2.z = 0
		vm.Positions = append(vm.Positions, move1)
		vm.Positions = append(vm.Positions, move2)
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
		vm.Positions = append(vm.Positions, move1)
		vm.Positions = append(vm.Positions, move2)
		vm.Positions = append(vm.Positions, move3)
		return
	}
}

// Generate move information
func (vm *Machine) Info() (minx, miny, minz, maxx, maxy, maxz float64, feedrates []float64) {
	for _, pos := range vm.Positions {
		if pos.x < minx {
			minx = pos.x
		} else if pos.x > maxx {
			maxx = pos.x
		}

		if pos.y < miny {
			miny = pos.y
		} else if pos.y > maxy {
			maxy = pos.y
		}

		if pos.z < minz {
			minz = pos.z
		} else if pos.z > maxz {
			maxz = pos.z
		}

		feedrateFound := false
		for _, feed := range feedrates {
			if feed == pos.state.feedrate {
				feedrateFound = true
				break
			}
		}
		if !feedrateFound {
			feedrates = append(feedrates, pos.state.feedrate)
		}
	}
	return
}
