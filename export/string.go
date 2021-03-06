package export

import "github.com/kennylevinsen/gocnc/vm"
import "fmt"
import "strings"

//
// String code generator
//
// Used for exporting VM state as a gcode string
//
// Notes:
//   Inverse time feed requires F to be set for every G-word, which is not done
//

type StringCodeGenerator struct {
	BaseGenerator
	Precision      int
	Lines          []string
	Tool           int
	ForceModeWrite bool
}

// Initializes state, and puts in a header block.
func (s *StringCodeGenerator) Init() {
	s.Position = vm.Position{State: vm.NewState()}
	s.Lines = []string{"(Exported by gocnc)", "G21G90\n"}
}

func (s *StringCodeGenerator) put(x string) {
	s.Lines = append(s.Lines, x)
}

// Fetch the generated gcodes.
func (s *StringCodeGenerator) Retrieve() string {
	return strings.Join(s.Lines, "\n")
}

// Adds a toolchange operation (M6 Tn).
func (s *StringCodeGenerator) ToolChange(t int) {
	if s.Tool == t {
		if s.Lines[len(s.Lines)-1] == fmt.Sprintf("T%d", t) {
			s.Lines[len(s.Lines)-1] = fmt.Sprintf("M6 T%d", t)
		} else {
			s.put("M6")
		}
	} else {
		s.put(fmt.Sprintf("M6 T%d", t))
		s.Tool = t
	}
	s.ForceModeWrite = true
}

// Adds a toolchange suggest operation (Tn).
func (s *StringCodeGenerator) ToolChangeSuggestion(t int) {
	if s.Tool != t {
		s.put(fmt.Sprintf("T%d", t))
		s.Tool = t
		s.ForceModeWrite = true
	}
}

// Adds a tool length index operation (G43 Hn or G49)
func (s *StringCodeGenerator) ToolLengthChange(h int) {
	switch h {
	case 0:
		s.put("G49")
	default:
		s.put(fmt.Sprintf("G43H%d", h))
	}
}

// Adds a spindle operation (M3/M4/M5 [Sn]).
func (s *StringCodeGenerator) Spindle(enabled, clockwise bool, speed float64) {
	x := ""
	if s.Position.State.SpindleEnabled != enabled || s.Position.State.SpindleClockwise != clockwise {
		s.ForceModeWrite = true
		if enabled && clockwise {
			x += "M3"
		} else if enabled && !clockwise {
			x += "M4"
		} else {
			x += "M5"
		}
	}

	if enabled && s.Position.State.SpindleSpeed != speed {
		x += fmt.Sprintf("S%s", floatToString(speed, s.Precision))
	}

	s.put(x)
}

// Adds a coolant operation (M7/M8/M9).
func (s *StringCodeGenerator) Coolant(floodCoolant, mistCoolant bool) {
	if !floodCoolant && !mistCoolant {
		s.put("M9")
	} else {
		if floodCoolant {
			s.put("M8")
		}
		if mistCoolant {
			s.put("M7")
		}
	}
	s.ForceModeWrite = true
}

// Sets feedmode (G93/G94/G95)
func (s *StringCodeGenerator) FeedMode(feedMode int) {
	switch feedMode {
	case vm.FeedModeInvTime:
		s.put("G93")
	case vm.FeedModeUnitsMin:
		s.put("G94")
	case vm.FeedModeUnitsRev:
		s.put("G95")
	default:
		panic("Unknown feed mode")
	}
}

// Sets feedrate (Fn)
func (s *StringCodeGenerator) Feedrate(feedrate float64) {
	s.put(fmt.Sprintf("F%s", floatToString(feedrate, s.Precision)))
}

// Sets cutter compensation mode (G40/G41/G42)
func (s *StringCodeGenerator) CutterCompensation(cutComp int) {
	switch cutComp {
	case vm.CutCompModeNone:
		s.put("G40")
	case vm.CutCompModeOuter:
		s.put("G41")
	case vm.CutCompModeInner:
		s.put("G42")
	default:
		panic("Unknown cutter compensation mode")
	}
}

func (s *StringCodeGenerator) Dwell(seconds float64) {
	s.put(fmt.Sprintf("G4P%s", floatToString(seconds, s.Precision)))
}

// Issues a move ([G0/G1] [Xn] [Yn] [Zn])
func (s *StringCodeGenerator) Move(x, y, z float64, moveMode int) {
	w := ""
	pos := s.GetPosition()
	if pos.State.MoveMode != moveMode || s.ForceModeWrite {
		switch moveMode {
		case vm.MoveModeNone:
			return
		case vm.MoveModeRapid:
			w = "G0"
		case vm.MoveModeLinear:
			w = "G1"
		case vm.MoveModeCWArc:
			panic("Cannot export arcs")
		case vm.MoveModeCCWArc:
			panic("Cannot export arcs")
		default:
			panic("Unknown move mode")
		}
	}

	s.ForceModeWrite = false

	if pos.X != x {
		w += fmt.Sprintf("X%s", floatToString(x, s.Precision))
	}
	if pos.Y != y {
		w += fmt.Sprintf("Y%s", floatToString(y, s.Precision))
	}
	if pos.Z != z {
		w += fmt.Sprintf("Z%s", floatToString(z, s.Precision))
	}

	s.put(w)
}
