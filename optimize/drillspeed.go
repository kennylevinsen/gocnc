package optimize

import "github.com/joushou/gocnc/vm"
import "github.com/joushou/gocnc/vector"

// OptDrillSpeed detects a previous drill, and uses rapid move or higher
// feedrate to the previous known depth. Scans through all Z-descent moves,
// logs its height, and ensures that any future move at that location will use
// vm.MoveModeRapid to go to the deepest previous known Z-height.
func OptDrillSpeed(machine *vm.Machine, feedrate float64, rapid bool) {
	var (
		last       vector.Vector
		npos       []vm.Position = make([]vm.Position, 0)
		drillStack []vm.Position = make([]vm.Position, 0)
	)

	fastDrill := func(pos vm.Position) (vm.Position, vm.Position, bool) {
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
				if rapid {
					pos.State.MoveMode = vm.MoveModeRapid
				} else {
					pos.State.Feedrate = feedrate
				}
				return pos, pos, false
			} else { // Can only rapid some of the way
				p := pos
				p.Z = depth

				if rapid {
					p.State.MoveMode = vm.MoveModeRapid
				} else {
					p.State.Feedrate = feedrate
				}
				return p, pos, true
			}
		} else {
			return pos, pos, false
		}
	}

	for _, m := range machine.Positions {
		if m.X == last.X && m.Y == last.Y && m.Z < last.Z && m.State.MoveMode == vm.MoveModeLinear {
			posn, poso, shouldinsert := fastDrill(m)
			if shouldinsert {
				npos = append(npos, posn)
			}
			npos = append(npos, poso)
		} else {
			npos = append(npos, m)
		}
		last = m.Vector()
	}
	machine.Positions = npos
}
