package optimizer

import "github.com/joushou/gocnc/gcode"

func FilemarkRemover(doc *gcode.GDocument) *gcode.GDocument {
	ndoc := gcode.GDocument{}
	var curBlock gcode.GBlock
	for _, block := range doc.Blocks {
		curBlock = gcode.GBlock{}
		for _, code := range block.Codes {
			if code.Type() != "filemarker" {
				curBlock.AppendCode(code)
			}
		}
		ndoc.AppendBlock(curBlock)
	}
	return &ndoc
}

func FeedratePatcher(doc *gcode.GDocument) *gcode.GDocument {
	ndoc := gcode.GDocument{}
	var curBlock gcode.GBlock
	var lastVal float64
	for _, block := range doc.Blocks {
		curBlock = gcode.GBlock{}
		for _, code := range block.Codes {
			if code.Address() == 'F' || code.Address() == 'S' {
				if code.Command() != lastVal {
					b := gcode.GBlock{}
					b.AppendCode(code)
					ndoc.AppendBlock(b)
					lastVal = code.Command()
				}
			} else {
				curBlock.AppendCode(code)
			}
		}
		ndoc.AppendBlock(curBlock)
	}
	return &ndoc
}
