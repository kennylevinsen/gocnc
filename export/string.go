package export

import "github.com/joushou/gocnc/vm"
import "fmt"

type StringCodeGenerator struct {
	BaseGenerator
	Precision int
	Lines     []string
}

// Initializes state, and puts in a header block.
func (s *StringCodeGenerator) Init() {
	s.Position = vm.Position{State: vm.State{0, 0, 0, -1, false, false, false, false, -1, -1}}
	s.Lines = []string{"(Exported by gocnc)", "G21G90\n"}
}

func (s *StringCodeGenerator) put(x string) {
	s.Lines = append(s.Lines, x)
}

// Fetch the generated gcodes.
func (s *StringCodeGenerator) Retrieve() string {
	z := ""
	for _, x := range s.Lines {
		z += fmt.Sprintf("%s\n", x)
	}
	return z
}

// Adds a toolchange operation (M6 Tn).
func (s *StringCodeGenerator) Toolchange(t int) {
	s.put(fmt.Sprintf("M6 T%d", t))
}

// Adds a spindle operation (M3/M4/M5 [Sn]).
func (s *StringCodeGenerator) Spindle(enabled, clockwise bool, speed float64) {
	x := ""
	if s.Position.State.SpindleEnabled != enabled || s.Position.State.SpindleClockwise != clockwise {
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

// Issues a move ([G0/G1] [Xn] [Yn] [Zn])
func (s *StringCodeGenerator) Move(x, y, z float64, moveMode int) {
	w := ""
	pos := s.GetPosition()
	if pos.State.MoveMode != moveMode {
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
