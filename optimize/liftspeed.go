package optimize

import "github.com/joushou/gocnc/vm"
import "github.com/joushou/gocnc/vector"

// Uses rapid move for all Z-up only moves.
// Scans all positions for moves that only change the z-axis in a positive direction,
// and sets the moveMode to vm.MoveModeRapid.
func OptLiftSpeed(machine *vm.Machine) {
	var last vector.Vector
	for idx, m := range machine.Positions {
		if m.X == last.X && m.Y == last.Y && m.Z > last.Z {
			// We got a lift! Let's make it faster, shall we?
			machine.Positions[idx].State.MoveMode = vm.MoveModeRapid
		}
		last = m.Vector()
	}
}
