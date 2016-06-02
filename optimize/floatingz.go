package optimize

import "github.com/joushou/gocnc/vm"

// OptFloatingZ eliminates any bogus moves above minDistOverZ, and makes the remaining move
// rapid.
//
// The machine positions are searched in pairs. If any of the two positions
// are below minDistOverZ, the pair is left untouched. Otherwise, the first
// position in the pair is removed, the movemode is set to rapid, and the
// remaining position is paired up with the next position. This process
// repeats until there are no positions left.
func OptFloatingZ(machine *vm.Machine, minDistOverZ float64) {
	mp := machine.Positions
	if len(mp) == 0 {
		return
	}

	npos := []vm.Position{mp[0]}
	for i := 1; i < len(mp); i++ {
		// We always append for the last position
		if i == len(mp)-1 {
			npos = append(npos, mp[i])
			continue
		}

		// This movement touches the material, skip 2 ahead.
		if !(mp[i].Z > minDistOverZ && npos[len(npos)-1].Z > minDistOverZ) {
			npos = append(npos, mp[i], mp[i+1])
			i++
			continue
		}

		mp[i].State.MoveMode = vm.MoveModeRapid
		npos[len(npos)-1] = mp[i]
	}

	machine.Positions = npos
}
