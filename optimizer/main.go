package optimizer

import "github.com/joushou/gocnc/gcode"
import "math"
import "fmt"

// Removes useless filemarks, comments, block-deletes and empty lines
func FillRemover(doc *gcode.Document) *gcode.Document {

	ndoc := gcode.Document{}
	var curBlock gcode.Block

	for _, block := range doc.Blocks {
		if block.BlockDelete || len(block.Nodes) == 0 {
			continue
		}
		curBlock = gcode.Block{}

		for _, node := range block.Nodes {
			switch node.(type) {
			case *gcode.Comment:
				continue
			case *gcode.Filemarker:
				continue
			default:
				curBlock.AppendNode(node)
			}
		}

		ndoc.AppendBlock(curBlock)
	}
	return &ndoc
}

// Ensures that feedrate is set on its own line,
func FeedratePatcher(doc *gcode.Document) *gcode.Document {

	ndoc := gcode.Document{}
	var (
		curBlock gcode.Block
		lastVal  float64
	)

	for _, block := range doc.Blocks {
		curBlock = gcode.Block{}

		for _, node := range block.Nodes {
			switch elem := node.(type) {
			case *gcode.Word:
				if elem.Address == 'F' || elem.Address == 'S' {
					if elem.Command != lastVal {
						b := gcode.Block{}
						b.AppendNode(elem)
						ndoc.AppendBlock(b)
						lastVal = elem.Command
					}
					continue
				}
			}
			curBlock.AppendNode(node)
		}
		ndoc.AppendBlock(curBlock)
	}
	return &ndoc
}

// Saves G0-3 codes in moves
func CodeSaver(doc *gcode.Document) *gcode.Document {
	ndoc := gcode.Document{}
	var (
		curBlock gcode.Block
		curWord  *gcode.Word
	)

	curWord = &gcode.Word{}

	for _, block := range doc.Blocks {
		curBlock = gcode.Block{}

		for _, node := range block.Nodes {
			switch elem := node.(type) {
			case *gcode.Word:
				if elem.Address == 'G' || elem.Address == 'M' {
					if curWord.Address == elem.Address && curWord.Command == elem.Command {
						continue
					} else if elem.Address == 'G' && elem.Command >= 0 && elem.Command <= 3 {
						curWord = elem
						curBlock.AppendNode(elem)
					} else {
						curWord = elem
						curBlock.AppendNode(elem)
					}

				} else {
					curBlock.AppendNode(elem)
				}
			default:
				curBlock.AppendNode(elem)
			}
		}
		ndoc.AppendBlock(curBlock)
	}
	return &ndoc
}

// Save silly double-moves
// TODO: FIX WHEN CHANGING MODE/FEEDRATE
func LinearMoveSaver(doc *gcode.Document) *gcode.Document {
	ndoc := gcode.Document{}

	var (
		xState, yState, zState float64
		lastX, lastY, lastZ    float64
	)
	noLast := true

	for _, block := range doc.Blocks {
		var nXState, nYState, nZState float64
		save := true

		// Update our state, and see if we may run
		for _, node := range block.Nodes {
			switch elem := node.(type) {
			case *gcode.Word:
				if elem.Address == 'X' {
					nXState = elem.Command
				} else if elem.Address == 'Y' {
					nYState = elem.Command
				} else if elem.Address == 'Z' {
					nZState = elem.Command
				} else {
					save = false
				}
			}
		}

		if !save {
			xState, yState, zState = nXState, nYState, nZState
			noLast = true
			ndoc.AppendBlock(block)
			continue
		} else {
			fmt.Printf("Time for math! %s\n", block.Export(-1))
		}

		dX, dY, dZ := nXState-xState, nYState-yState, nZState-zState

		xState, yState, zState = nXState, nYState, nZState

		if dX == 0 && dY == 0 && dZ == 0 {
			// This isn't even a move...
			continue
		}

		norm := math.Sqrt(math.Pow(dX, 2) + math.Pow(dY, 2) + math.Pow(dZ, 2))
		vecX, vecY, vecZ := dX/norm, dY/norm, dZ/norm

		if !noLast && lastX == vecX && lastY == vecY && lastZ == vecZ {
			ndoc.Blocks = ndoc.Blocks[:len(ndoc.Blocks)-2]
		} else {
			noLast = false
			lastX, lastY, lastZ = vecX, vecY, vecZ
		}

		ndoc.AppendBlock(block)
	}
	return &ndoc

}
