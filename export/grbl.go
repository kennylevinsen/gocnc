package export

import "github.com/joushou/gocnc/vm"
import "fmt"

type GrblGenerator struct {
	BaseGenerator
	Precision      int
	Write          func(string)
	ForceModeWrite bool
}

// A no-op toolchange, as Grbl doesn't support it
func (s *GrblGenerator) Toolchange(t int) {
	// TODO Implement manual tool-change
}

func (s *GrblGenerator) Spindle(enabled, clockwise bool, speed float64) {
	state := s.Position.State
	x := ""
	if state.SpindleEnabled != enabled || state.SpindleClockwise != clockwise {
		s.ForceModeWrite = true
		if enabled && clockwise {
			x += "M3"
		} else if enabled && !clockwise {
			x += "M4"
		} else {
			x += "M5"
		}
	}

	if enabled && state.SpindleSpeed != speed {
		x += fmt.Sprintf("S%s", floatToString(speed, s.Precision))
	}
	s.Write(x)
}

func (s *GrblGenerator) Coolant(floodCoolant, mistCoolant bool) {
	if !floodCoolant && !mistCoolant {
		s.Write("M9")
	} else {
		if floodCoolant {
			s.Write("M8")
		}
		if mistCoolant {
			s.Write("M7")
		}
	}
	s.ForceModeWrite = true
}

func (s *GrblGenerator) FeedMode(feedMode int) {
	switch feedMode {
	case vm.FeedModeInvTime:
		s.Write("G93")
	case vm.FeedModeUnitsMin:
		s.Write("G94")
	case vm.FeedModeUnitsRev:
		s.Write("G95")
	default:
		panic("Unknown feed mode")
	}
}

func (s *GrblGenerator) Feedrate(feedrate float64) {
	s.Write(fmt.Sprintf("F%s", floatToString(feedrate, s.Precision)))
}

// A no-op cutter-compensation, as Grbl doesn't support it
func (s *GrblGenerator) CutterCompensation(cutComp int) {
	if cutComp != vm.CutCompModeNone {
		panic("Cutter compensation not supported by Grbl")
	}
}

func (s *GrblGenerator) Move(x, y, z float64, moveMode int) {
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

	s.Write(w)
}
