package optimize

import "github.com/joushou/gocnc/vm"

// Uses rapid move for all Z-up only moves.
// Scans all positions for moves that only change the z-axis in a positive direction,
// and sets the moveMode to vm.MoveModeRapid.
func OptLiftSpeed(machine *vm.Machine) {
	var lastx, lasty, lastz float64
	for idx, m := range machine.Positions {
		if m.X == lastx && m.Y == lasty && m.Z > lastz {
			// We got a lift! Let's make it faster, shall we?
			machine.Positions[idx].State.MoveMode = vm.MoveModeRapid
		}
		lastx, lasty, lastz = m.X, m.Y, m.Z
	}
}
