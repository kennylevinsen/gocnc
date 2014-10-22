package vm

import "strconv"
import "strings"
import "fmt"

//
// Export machine code
//

func (vm *Machine) Export(precision int) string {
	blocks := make([]string, 0)
	blocks = append(blocks, "(Exported by gocnc/vm)")
	blocks = append(blocks, "G17G21G40G90G94")

	var lastFeedrate, lastSpindleSpeed, lastX, lastY, lastZ float64
	var spindleEnabled, spindleClockwise, mistCoolant, floodCoolant bool
	lastMoveMode := ""

	for _, pos := range vm.posStack {
		s := pos.state
		var x, moveMode string

		// fetch move mode
		switch s.moveMode {
		case initialMode:
			continue
		case rapidMoveMode:
			moveMode = "G0"
		case linearMoveMode:
			moveMode = "G1"
		case cwArcMode:
			moveMode = "G2"
		case ccwArcMode:
			moveMode = "G3"
		}

		// handle spindle
		if s.spindleEnabled != spindleEnabled || s.spindleClockwise != spindleClockwise {
			if s.spindleEnabled && s.spindleClockwise {
				blocks = append(blocks, "M3")
			} else if s.spindleEnabled && !s.spindleClockwise {
				blocks = append(blocks, "M4")
			} else if !s.spindleEnabled {
				blocks = append(blocks, "M5")
			}
			spindleEnabled, spindleClockwise = s.spindleEnabled, s.spindleClockwise
			lastMoveMode = "" // M-codes clear stuff...
		}

		// handle coolant
		if s.floodCoolant != floodCoolant || s.mistCoolant != mistCoolant {

			if (floodCoolant == true && s.floodCoolant == false) ||
				(mistCoolant == true && s.mistCoolant == false) {
				// We can only disable both coolants simultaneously, so kill it and reenable one if needed
				blocks = append(blocks, "M9")
			}
			if s.floodCoolant {
				blocks = append(blocks, "M8")
			} else if s.mistCoolant {
				blocks = append(blocks, "M7")
			}
			lastMoveMode = "" // M-codes clear stuff...
		}

		// handle feedrate
		if s.moveMode != rapidMoveMode {
			if s.feedrate != lastFeedrate {
				blocks = append(blocks, "F"+strconv.FormatFloat(s.feedrate, 'f', precision, 64))
				lastFeedrate = s.feedrate
			}
		}

		// handle spindle speed
		if s.spindleSpeed != lastSpindleSpeed {
			blocks = append(blocks, "S"+strconv.FormatFloat(s.spindleSpeed, 'f', precision, 64))
			lastSpindleSpeed = s.spindleSpeed
		}

		// handle move mode
		if s.moveMode == cwArcMode || s.moveMode == ccwArcMode || moveMode != lastMoveMode {
			x += moveMode
			//blocks = append(blocks, moveMode)
			lastMoveMode = moveMode
		}

		// handle move
		if pos.x != lastX {
			x += "X" + strconv.FormatFloat(pos.x, 'f', precision, 64)
			lastX = pos.x
		}
		if pos.y != lastY {
			x += "Y" + strconv.FormatFloat(pos.y, 'f', precision, 64)
			lastY = pos.y
		}
		if pos.z != lastZ {
			x += "Z" + strconv.FormatFloat(pos.z, 'f', precision, 64)
			lastZ = pos.z
		}

		if s.moveMode == cwArcMode || s.moveMode == ccwArcMode {
			if pos.i != 0 {
				x += "I" + strconv.FormatFloat(pos.i, 'f', precision, 64)
			}
			if pos.j != 0 {
				x += "J" + strconv.FormatFloat(pos.j, 'f', precision, 64)
			}
			if pos.k != 0 {
				x += "K" + strconv.FormatFloat(pos.k, 'f', precision, 64)
			}
			if pos.rot != 1 {
				x += "P" + strconv.FormatInt(pos.rot, 10)
			}
		}

		// put on slice
		if len(x) > 0 {
			blocks = append(blocks, x)
		}
	}
	return strings.Join(blocks, "\n")
}

//
// Dump moves in (sort of) human readable format
//
func (vm *Machine) Dump() {
	for _, m := range vm.posStack {
		switch m.state.moveMode {
		case initialMode:
			fmt.Printf("initial pos, ")
		case rapidMoveMode:
			fmt.Printf("rapid move, ")
		case linearMoveMode:
			fmt.Printf("linear move, ")
		case cwArcMode:
			fmt.Printf("clockwise arc, ")
		case ccwArcMode:
			fmt.Printf("counterclockwise arc, ")
		}

		fmt.Printf("feedrate: %f, ", m.state.feedrate)
		fmt.Printf("spindle: %f, ", m.state.spindleSpeed)
		fmt.Printf("X: %f, Y: %f, Z: %f, I: %f, J: %f, K: %f\n", m.x, m.y, m.z, m.i, m.j, m.k)
	}
}
