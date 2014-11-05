package vm

import "github.com/joushou/gocnc/gcode"
import "errors"
import "fmt"

//
// Export machine code
//

func (vm *Machine) Export() (res *gcode.Document, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New(fmt.Sprintf("%s", r))
		}
	}()
	var (
		lastFeedrate, lastSpindleSpeed, lastX, lastY, lastZ         float64
		spindleEnabled, spindleClockwise, mistCoolant, floodCoolant bool
		lastMoveMode, lastFeedMode, lastCutCompMode                 int = 0, 0, -1
		lastTool                                                    int
		doc                                                         gcode.Document
	)

	shortBlock := func(ns ...gcode.Node) {
		var block gcode.Block
		for _, n := range ns {
			block.AppendNode(n)
		}
		doc.AppendBlock(block)
	}

	shortBlock(&gcode.Comment{"Exported by gocnc/vm", false})
	shortBlock(&gcode.Word{'G', 21}, &gcode.Word{'G', 90})

	for _, pos := range vm.Positions {
		s := pos.State

		// select tool
		if s.Tool != lastTool {
			shortBlock(&gcode.Word{'M', 6}, &gcode.Word{'T', float64(s.Tool)})
			lastTool = s.Tool
		}

		// handle spindle
		if s.SpindleEnabled != spindleEnabled || s.SpindleClockwise != spindleClockwise {
			if s.SpindleEnabled && s.SpindleClockwise {
				shortBlock(&gcode.Word{'M', 3})
			} else if s.SpindleEnabled && !s.SpindleClockwise {
				shortBlock(&gcode.Word{'M', 4})
			} else if !s.SpindleEnabled {
				shortBlock(&gcode.Word{'M', 5})
			}
			spindleEnabled, spindleClockwise = s.SpindleEnabled, s.SpindleClockwise
			lastMoveMode = -1 // M-codes clear stuff...
		}

		// handle coolant
		if s.FloodCoolant != floodCoolant || s.MistCoolant != mistCoolant {

			if (floodCoolant == true && s.FloodCoolant == false) ||
				(mistCoolant == true && s.MistCoolant == false) {
				// We can only disable both coolants simultaneously, so kill it and reenable one if needed
				shortBlock(&gcode.Word{'M', 9})
				mistCoolant, floodCoolant = false, false
			}
			if s.FloodCoolant {
				shortBlock(&gcode.Word{'M', 8})
				floodCoolant = true

			} else if s.MistCoolant {
				shortBlock(&gcode.Word{'M', 7})
				mistCoolant = true
			}
			lastMoveMode = -1 // M-codes clear stuff...
		}

		// handle feedrate mode
		if s.FeedMode != lastFeedMode {
			switch s.FeedMode {
			case FeedModeInvTime:
				shortBlock(&gcode.Word{'G', 93})
			case FeedModeUnitsMin:
				shortBlock(&gcode.Word{'G', 94})
			case FeedModeUnitsRev:
				shortBlock(&gcode.Word{'G', 95})
			}
			lastFeedMode = s.FeedMode
			lastFeedrate = -1 // Clears feedrate
		}

		// handle feedrate and spindle speed
		if s.MoveMode != MoveModeRapid {
			if s.Feedrate != lastFeedrate {
				shortBlock(&gcode.Word{'F', s.Feedrate})
				lastFeedrate = s.Feedrate
			}
			if s.SpindleSpeed != lastSpindleSpeed {
				shortBlock(&gcode.Word{'S', s.SpindleSpeed})
				lastSpindleSpeed = s.SpindleSpeed
			}
		}

		// handle cutter compensation
		if s.CutterCompensation != lastCutCompMode {
			switch s.CutterCompensation {
			case CutCompModeNone:
				shortBlock(&gcode.Word{'G', 40})
			case CutCompModeOuter:
				shortBlock(&gcode.Word{'G', 41})
			case CutCompModeInner:
				shortBlock(&gcode.Word{'G', 42})
			}
			lastCutCompMode = s.CutterCompensation
		}

		var moveBlock gcode.Block

		// handle move mode
		if s.MoveMode != lastMoveMode {
			switch s.MoveMode {
			case MoveModeNone:
				continue
			case MoveModeRapid:
				moveBlock.AppendNode(&gcode.Word{'G', 0})
			case MoveModeLinear:
				moveBlock.AppendNode(&gcode.Word{'G', 1})
			default:
				panic("Cannot export arcs")
			}
			lastMoveMode = s.MoveMode
		}

		// handle move
		if pos.X != lastX {
			moveBlock.AppendNode(&gcode.Word{'X', pos.X})
			lastX = pos.X
		}
		if pos.Y != lastY {
			moveBlock.AppendNode(&gcode.Word{'Y', pos.Y})
			lastY = pos.Y
		}
		if pos.Z != lastZ {
			moveBlock.AppendNode(&gcode.Word{'Z', pos.Z})
			lastZ = pos.Z
		}

		// put on slice
		if len(moveBlock.Nodes) > 0 {
			doc.AppendBlock(moveBlock)
		}
	}
	return &doc, nil
}

//
// Dump moves in (sort of) human readable format
//
func (m *Position) Dump() {
	switch m.State.MoveMode {
	case MoveModeNone:
		fmt.Printf("Null move\n")
	case MoveModeRapid:
		fmt.Printf("Rapid move\n")
	case MoveModeLinear:
		fmt.Printf("Linear move\n")
	case MoveModeCWArc:
		fmt.Printf("Clockwise arc\n")
	case MoveModeCCWArc:
		fmt.Printf("Counterclockwise arc\n")
	}
	fmt.Printf("   Feedrate: %g\n", m.State.Feedrate)
	fmt.Printf("   Spindle: %t, clockwise: %t, speed: %g\n", m.State.SpindleEnabled, m.State.SpindleClockwise, m.State.SpindleSpeed)
	fmt.Printf("   Mist coolant: %t, flood coolant: %t\n", m.State.MistCoolant, m.State.FloodCoolant)
	fmt.Printf("   X: %f, Y: %f, Z: %f\n", m.X, m.Y, m.Z)
}

func (vm *Machine) Dump() {
	for _, m := range vm.Positions {
		m.Dump()
	}
}
