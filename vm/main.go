package vm

import "github.com/joushou/gocnc/gcode"

type Statement map[rune]float64

type VM struct {
}

func (vm *VM) run(stmt Statement) {
}

func (vm *VM) Process(doc *gcode.Document) {
	var stmt Statement

	for _, b := range doc.Blocks {
		if b.BlockDelete {
			continue
		}

		for _, n := range b.Nodes {
			if word, ok := n.(*gcode.Word); ok {
				stmt[word.Address] = word.Command
			}
		}
		vm.run(stmt)
	}
}
