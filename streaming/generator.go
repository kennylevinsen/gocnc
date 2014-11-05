package streaming

import "github.com/joushou/gocnc/vm"
import "strconv"
import "strings"
import "fmt"

func floatToString(f float64, p int) string {
	x := strconv.FormatFloat(f, 'f', p, 64)

	// Hacky way to remove silly zeroes
	if strings.IndexRune(x, '.') != -1 {
		for x[len(x)-1] == '0' {
			x = x[:len(x)-1]
		}
		if x[len(x)-1] == '.' {
			x = x[:len(x)-1]
		}
	}

	return x
}

type CodeGenerator interface {
	Toolchange(int) string
	Spindle(bool, bool, float64) string
	Coolant(bool, bool) string
	FeedMode(int) string
	Feedrate(float64) string
	CutterCompensation(int) string
	Move(float64, float64, float64, int) string
	Init()
}

type StandardGenerator struct {
	state               vm.State
	Precision           int
	lastx, lasty, lastz float64
}

func (s *StandardGenerator) Toolchange(t int) string {
	defer func() {
		s.state.Tool = t
	}()
	if s.state.Tool != t {
		return fmt.Sprintf("M6 T%d", t)
	}
	return ""
}

func (s *StandardGenerator) Spindle(enabled, clockwise bool, speed float64) string {
	defer func() {
		s.state.SpindleEnabled, s.state.SpindleClockwise, s.state.SpindleSpeed = enabled, clockwise, speed
	}()

	if s.state.SpindleEnabled == enabled && s.state.SpindleClockwise == clockwise && s.state.SpindleSpeed == speed {
		return ""
	}

	x := ""
	if s.state.SpindleEnabled != enabled || s.state.SpindleClockwise != clockwise {
		if enabled && clockwise {
			x += "M3"
		} else if enabled && !clockwise {
			x += "M4"
		} else {
			x += "M5"
		}
	}

	if enabled && s.state.SpindleSpeed != speed {
		x += fmt.Sprintf("S%s", floatToString(speed, s.Precision))
	}

	return x
}

func (s *StandardGenerator) Coolant(floodCoolant, mistCoolant bool) string {
	defer func() {
		s.state.FloodCoolant, s.state.MistCoolant = floodCoolant, mistCoolant
	}()

	if s.state.FloodCoolant == floodCoolant && s.state.MistCoolant == mistCoolant {
		return ""
	}

	if !floodCoolant && !mistCoolant {
		return "M9"
	} else {
		if floodCoolant {
			return "M8"
		}
		if mistCoolant {
			return "M7"
		}
	}
	return ""
}

func (s *StandardGenerator) FeedMode(feedMode int) string {
	defer func() {
		s.state.FeedMode = feedMode
	}()

	if s.state.FeedMode == feedMode {
		return ""
	}

	switch feedMode {
	case vm.FeedModeInvTime:
		return "G93"
	case vm.FeedModeUnitsMin:
		return "G94"
	case vm.FeedModeUnitsRev:
		return "G95"
	default:
		return ""
	}
	panic("Unknown feed mode")
}

func (s *StandardGenerator) Feedrate(feedrate float64) string {
	defer func() {
		s.state.Feedrate = feedrate
	}()

	if s.state.Feedrate == feedrate {
		return ""
	}

	return fmt.Sprintf("F%s", floatToString(feedrate, s.Precision))
}

func (s *StandardGenerator) CutterCompensation(cutComp int) string {
	defer func() {
		s.state.CutterCompensation = cutComp
	}()

	if s.state.CutterCompensation == cutComp {
		return ""
	}

	switch cutComp {
	case vm.CutCompModeNone:
		return "G40"
	case vm.CutCompModeOuter:
		return "G41"
	case vm.CutCompModeInner:
		return "G42"
	}
	panic("Unknown cutter compensation mode")
}

func (s *StandardGenerator) Move(x, y, z float64, moveMode int) string {
	defer func() {
		s.lastx, s.lasty, s.lastz = x, y, z
		s.state.MoveMode = moveMode
	}()
	w := ""
	if s.state.MoveMode != moveMode {
		switch moveMode {
		case vm.MoveModeRapid:
			w = "G0"
		case vm.MoveModeLinear:
			w = "G1"
		case vm.MoveModeCWArc:
			panic("Cannot export arcs")
		case vm.MoveModeCCWArc:
			panic("Cannot export arcs")
		default:
			return ""
		}
	}
	if s.lastx != x {
		w += fmt.Sprintf("X%s", floatToString(x, s.Precision))
	}
	if s.lasty != y {
		w += fmt.Sprintf("Y%s", floatToString(y, s.Precision))
	}
	if s.lastz != z {
		w += fmt.Sprintf("Z%s", floatToString(z, s.Precision))
	}

	return w
}

func (s *StandardGenerator) Init() {
	s.state = vm.State{0, 0, 0, -1, false, false, false, false, -1, -1}
}

func HandlePosition(s CodeGenerator, pos vm.Position) []string {
	ss := pos.State
	res := make([]string, 0)
	if x := s.Toolchange(ss.Tool); len(x) > 0 {
		res = append(res, x)
	}
	if x := s.Spindle(ss.SpindleEnabled, ss.SpindleClockwise, ss.SpindleSpeed); len(x) > 0 {
		res = append(res, x)
	}
	if x := s.Coolant(ss.FloodCoolant, ss.MistCoolant); len(x) > 0 {
		res = append(res, x)
	}
	if x := s.FeedMode(ss.FeedMode); len(x) > 0 {
		res = append(res, x)
	}
	if x := s.Feedrate(ss.Feedrate); len(x) > 0 {
		res = append(res, x)
	}
	if x := s.CutterCompensation(ss.CutterCompensation); len(x) > 0 {
		res = append(res, x)
	}
	if x := s.Move(pos.X, pos.Y, pos.Z, ss.MoveMode); len(x) > 0 {
		res = append(res, x)
	}
	return res
}

func Export(s CodeGenerator, m *vm.Machine) string {
	x := "(Exported by gocnc)\nG21G90\n"
	for _, p := range m.Positions {
		for _, k := range HandlePosition(s, p) {
			x += fmt.Sprintf("%s\n", k)
		}
	}
	return x
}
