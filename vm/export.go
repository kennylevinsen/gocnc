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
		lastMoveMode, lastFeedMode, lastCutCompMode                 float64 = -1, -1, -1
		lastTool                                                    int
		doc                                                         gcode.Document
	)

	shortBlockDesc := func(desc string, ns ...gcode.Node) {
		var block gcode.Block
		block.Description = desc
		for _, n := range ns {
			block.AppendNode(n)
		}
		doc.AppendBlock(block)
	}

	shortBlockDesc("comment", &gcode.Comment{"Exported by gocnc/vm", false})
	shortBlockDesc("header", &gcode.Word{'G', 21}, &gcode.Word{'G', 90})

	for _, pos := range vm.Positions {
		s := pos.State
		var moveMode, feedMode, cutCompMode float64

		// fetch cutter compensation mode
		switch s.CutterCompensation {
		case CutCompModeNone:
			cutCompMode = 40
		case CutCompModeOuter:
			cutCompMode = 41
		case CutCompModeInner:
			cutCompMode = 42
		}

		// fetch move mode
		switch s.MoveMode {
		case MoveModeNone:
			continue
		case MoveModeRapid:
			moveMode = 0
		case MoveModeLinear:
			moveMode = 1
		default:
			panic("Cannot export arcs")
		}

		// fetch feed mode
		switch s.FeedMode {
		case FeedModeInvTime:
			feedMode = 93
		case FeedModeUnitsMin:
			feedMode = 94
		case FeedModeUnitsRev:
			feedMode = 95
		}

		// select tool
		if s.Tool != lastTool {
			shortBlockDesc("tool-change", &gcode.Word{'M', 6}, &gcode.Word{'T', float64(s.Tool)})
			lastTool = s.Tool
		}

		// handle spindle
		if s.SpindleEnabled != spindleEnabled || s.SpindleClockwise != spindleClockwise {
			if s.SpindleEnabled && s.SpindleClockwise {
				shortBlockDesc("spindle-clockwise", &gcode.Word{'M', 3})
			} else if s.SpindleEnabled && !s.SpindleClockwise {
				shortBlockDesc("spindle-cclockwise", &gcode.Word{'M', 4})
			} else if !s.SpindleEnabled {
				shortBlockDesc("spindle-disabled", &gcode.Word{'M', 5})
			}
			spindleEnabled, spindleClockwise = s.SpindleEnabled, s.SpindleClockwise
			lastMoveMode = -1 // M-codes clear stuff...
		}

		// handle coolant
		if s.FloodCoolant != floodCoolant || s.MistCoolant != mistCoolant {

			if (floodCoolant == true && s.FloodCoolant == false) ||
				(mistCoolant == true && s.MistCoolant == false) {
				// We can only disable both coolants simultaneously, so kill it and reenable one if needed
				shortBlockDesc("disable-coolant", &gcode.Word{'M', 9})
				mistCoolant, floodCoolant = false, false
			}
			if s.FloodCoolant {
				shortBlockDesc("flood-coolant", &gcode.Word{'M', 8})
				floodCoolant = true

			} else if s.MistCoolant {
				shortBlockDesc("mist-coolant", &gcode.Word{'M', 7})
				mistCoolant = true
			}
			lastMoveMode = -1 // M-codes clear stuff...
		}

		// handle feedrate mode
		if feedMode != lastFeedMode {
			shortBlockDesc("feed-mode", &gcode.Word{'G', feedMode})
			lastFeedMode = feedMode
			lastFeedrate = -1 // Clears feedrate
		}

		// handle feedrate and spindle speed
		if s.MoveMode != MoveModeRapid {
			if s.Feedrate != lastFeedrate {
				shortBlockDesc("feedrate", &gcode.Word{'F', s.Feedrate})
				lastFeedrate = s.Feedrate
			}
			if s.SpindleSpeed != lastSpindleSpeed {
				shortBlockDesc("spindle-speed", &gcode.Word{'S', s.SpindleSpeed})
				lastSpindleSpeed = s.SpindleSpeed
			}
		}

		// handle cutter compensation
		if cutCompMode != lastCutCompMode {
			if cutCompMode == 40 {
				shortBlockDesc("cutter-compensation-reset", &gcode.Word{'G', cutCompMode})
			} else {
				shortBlockDesc("cutter-compensation-set", &gcode.Word{'G', cutCompMode})
			}
			lastCutCompMode = cutCompMode
		}

		var moveBlock gcode.Block
		moveBlock.Description = "move"

		// handle move mode
		if moveMode != lastMoveMode {
			moveBlock.AppendNode(&gcode.Word{'G', moveMode})
			lastMoveMode = moveMode
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
func (vm *Machine) Dump() {
	for _, m := range vm.Positions {
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
}
