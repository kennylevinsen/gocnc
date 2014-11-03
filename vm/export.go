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

	for _, pos := range vm.posStack {
		s := pos.state
		var moveMode, feedMode, cutCompMode float64

		// fetch cutter compensation mode
		switch s.cutterCompensation {
		case cutCompModeNone:
			cutCompMode = 40
		case cutCompModeOuter:
			cutCompMode = 41
		case cutCompModeInner:
			cutCompMode = 42
		}

		// fetch move mode
		switch s.moveMode {
		case moveModeNone:
			continue
		case moveModeRapid:
			moveMode = 0
		case moveModeLinear:
			moveMode = 1
		default:
			panic("Cannot export arcs")
		}

		// fetch feed mode
		switch s.feedMode {
		case feedModeInvTime:
			feedMode = 93
		case feedModeUnitsMin:
			feedMode = 94
		case feedModeUnitsRev:
			feedMode = 95
		}

		// select tool
		if s.tool != lastTool {
			shortBlockDesc("tool-change", &gcode.Word{'M', 6}, &gcode.Word{'T', float64(s.tool)})
			lastTool = s.tool
		}

		// handle spindle
		if s.spindleEnabled != spindleEnabled || s.spindleClockwise != spindleClockwise {
			if s.spindleEnabled && s.spindleClockwise {
				shortBlockDesc("spindle-clockwise", &gcode.Word{'M', 3})
			} else if s.spindleEnabled && !s.spindleClockwise {
				shortBlockDesc("spindle-cclockwise", &gcode.Word{'M', 4})
			} else if !s.spindleEnabled {
				shortBlockDesc("spindle-disabled", &gcode.Word{'M', 5})
			}
			spindleEnabled, spindleClockwise = s.spindleEnabled, s.spindleClockwise
			lastMoveMode = -1 // M-codes clear stuff...
		}

		// handle coolant
		if s.floodCoolant != floodCoolant || s.mistCoolant != mistCoolant {

			if (floodCoolant == true && s.floodCoolant == false) ||
				(mistCoolant == true && s.mistCoolant == false) {
				// We can only disable both coolants simultaneously, so kill it and reenable one if needed
				shortBlockDesc("disable-coolant", &gcode.Word{'M', 9})
				mistCoolant, floodCoolant = false, false
			}
			if s.floodCoolant {
				shortBlockDesc("flood-coolant", &gcode.Word{'M', 8})
				floodCoolant = true

			} else if s.mistCoolant {
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
		if s.moveMode != moveModeRapid {
			if s.feedrate != lastFeedrate {
				shortBlockDesc("feedrate", &gcode.Word{'F', s.feedrate})
				lastFeedrate = s.feedrate
			}
			if s.spindleSpeed != lastSpindleSpeed {
				shortBlockDesc("spindle-speed", &gcode.Word{'S', s.spindleSpeed})
				lastSpindleSpeed = s.spindleSpeed
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
		if s.moveMode == moveModeCWArc || s.moveMode == moveModeCCWArc || moveMode != lastMoveMode {
			moveBlock.AppendNode(&gcode.Word{'G', moveMode})
			lastMoveMode = moveMode
		}

		// handle move
		if pos.x != lastX {
			moveBlock.AppendNode(&gcode.Word{'X', pos.x})
			lastX = pos.x
		}
		if pos.y != lastY {
			moveBlock.AppendNode(&gcode.Word{'Y', pos.y})
			lastY = pos.y
		}
		if pos.z != lastZ {
			moveBlock.AppendNode(&gcode.Word{'Z', pos.z})
			lastZ = pos.z
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
	for _, m := range vm.posStack {
		switch m.state.moveMode {
		case moveModeNone:
			fmt.Printf("Null move\n")
		case moveModeRapid:
			fmt.Printf("Rapid move\n")
		case moveModeLinear:
			fmt.Printf("Linear move\n")
		case moveModeCWArc:
			fmt.Printf("Clockwise arc\n")
		case moveModeCCWArc:
			fmt.Printf("Counterclockwise arc\n")
		}
		fmt.Printf("   Feedrate: %f\n", m.state.feedrate)
		fmt.Printf("   Spindle: %t, clockwise: %t, speed: %f\n", m.state.spindleEnabled, m.state.spindleClockwise, m.state.spindleSpeed)
		fmt.Printf("   Mist coolant: %t, flood coolant: %t\n", m.state.mistCoolant, m.state.floodCoolant)
		fmt.Printf("   X: %f, Y: %f, Z: %f\n", m.x, m.y, m.z)
	}
}
