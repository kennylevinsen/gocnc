package optimize

import "github.com/joushou/gocnc/vm"
import "github.com/joushou/gocnc/vector"

// Kills redundant partial moves.
// Calculates the unit-vector, and kills all incremental moves between A and B.
// Deprecated by OptVector.
func OptBogusMoves(machine *vm.Machine) {
	var (
		lastvec vector.Vector
		state   vector.Vector
		npos    []vm.Position = make([]vm.Position, 0)
	)

	for _, m := range machine.Positions {
		d := m.Vector().Diff(state)
		state = m.Vector()

		if m.State.MoveMode != vm.MoveModeRapid && m.State.MoveMode != vm.MoveModeLinear {
			lastvec = vector.Vector{}
			continue
		}

		if d.X == 0 && d.Y == 0 && d.Z == 0 {
			// Why are we doing this again?!
			continue
		}

		norm := d.Norm()
		vec := d.Divide(norm)

		if vec == lastvec {
			npos[len(npos)-1] = m
		} else {
			npos = append(npos, m)
			lastvec = vec
		}
	}
	machine.Positions = npos
}
