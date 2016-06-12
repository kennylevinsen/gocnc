package optimize

import "github.com/joushou/gocnc/vm"

func OptPrepareTool(machine *vm.Machine) {
	lastTool := -1
	lastToolIdx := 0
	mp := machine.Positions

	for i := range mp {
		if mp[i].State.ToolIndex != lastTool {
			lastTool = mp[i].State.ToolIndex
			z := mp[lastToolIdx:i]
			for y := range z {
				z[y].State.NextToolIndex = lastTool
			}
			lastToolIdx = i
		}
	}
}
