package optimize

import "github.com/joushou/gocnc/vm"
import "github.com/joushou/gocnc/utils"

// Kills redundant partial moves.
// Calculates the unit-vector, and kills all incremental moves between A and B.
func OptVector(machine *vm.Machine, tolerance float64) {
	var (
		vec1, vec2, vec3 utils.Vector
		ready            int
		length1, length2 float64
		lastMoveMode     int
		npos             []vm.Position = make([]vm.Position, 0)
	)

	for _, m := range machine.Positions {
		if m.State.MoveMode != vm.MoveModeLinear && m.State.MoveMode != vm.MoveModeRapid {
			ready = 0
			goto appendpos
		}

		if m.State.MoveMode != lastMoveMode {
			lastMoveMode = m.State.MoveMode
			ready = 0
		}

		if ready == 0 {
			vec1 = m.Vector()
			ready++
			goto appendpos
		} else if ready == 1 {
			vec2 = m.Vector()
			ready++
			goto appendpos
		} else if ready == 2 {
			vec3 = m.Vector()
			ready++
		} else {
			vec1 = vec2
			vec2 = vec3
			vec3 = m.Vector()
		}

		length1 = vec1.Diff(vec2).Norm() + vec2.Diff(vec3).Norm()
		length2 = vec1.Diff(vec3).Norm()
		if length1-length2 < tolerance {
			npos[len(npos)-1] = m
			vec2 = vec1
			continue
		}

	appendpos:
		npos = append(npos, m)
	}
	machine.Positions = npos
}
