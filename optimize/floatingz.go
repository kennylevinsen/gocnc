package optimize

import "github.com/joushou/gocnc/vm"

// Eliminates any bogus moves above Z0
func OptFloatingZ(machine *vm.Machine) {
	var last vm.Position
	npos := make([]vm.Position, 0)

	for _, m := range machine.Positions {
		if last.Z > 0 && m.Z > 0 {
			if m.Z > npos[len(npos)-1].Z {
				npos[len(npos)-1].Z = m.Z
			}
		} else if last.Z > 0 && m.Z < 0 {
			npos = append(npos, last, m)
		} else {
			npos = append(npos, m)
		}
		last = m
	}
	machine.Positions = npos
}
