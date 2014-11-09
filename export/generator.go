package export

import "github.com/joushou/gocnc/vm"
import "strconv"
import "strings"

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

// Interface for exporting a vm position stack.
type CodeGenerator interface {
	GetPosition() vm.Position
	SetPosition(vm.Position)
	Toolchange(int)
	Spindle(bool, bool, float64)
	Coolant(bool, bool)
	FeedMode(int)
	Feedrate(float64)
	CutterCompensation(int)
	Move(float64, float64, float64, int)
	Init()
}

// A simple generator with a few essentials.
type BaseGenerator struct {
	Position vm.Position
}

// Gets the current position for comparisons.
func (s *BaseGenerator) GetPosition() vm.Position {
	return s.Position
}

// Sets the current position.
func (s *BaseGenerator) SetPosition(pos vm.Position) {
	s.Position = pos
}

// Initializes the current position.
func (s *BaseGenerator) Init() {
	s.Position = vm.Position{State: vm.State{0, 0, 0, -1, false, false, false, false, -1, -1}}
}

// Calls the CodeGenerator for all changed states.
func HandlePosition(s CodeGenerator, pos vm.Position) {
	cp := s.GetPosition()
	cs := cp.State
	ns := pos.State

	if ns.Tool != cs.Tool {
		s.Toolchange(ns.Tool)
	}

	if ns.SpindleEnabled != cs.SpindleEnabled ||
		ns.SpindleClockwise != cs.SpindleClockwise ||
		ns.SpindleSpeed != cs.SpindleSpeed {
		s.Spindle(ns.SpindleEnabled, ns.SpindleClockwise, ns.SpindleSpeed)
	}

	if ns.FloodCoolant != cs.FloodCoolant || ns.MistCoolant != cs.MistCoolant {
		s.Coolant(ns.FloodCoolant, ns.MistCoolant)
	}

	if ns.FeedMode != cs.FeedMode {
		s.FeedMode(ns.FeedMode)
	}

	if ns.Feedrate != cs.Feedrate {
		s.Feedrate(ns.Feedrate)
	}

	if ns.CutterCompensation != cs.CutterCompensation {
		s.CutterCompensation(ns.CutterCompensation)
	}

	if cp.X != pos.X || cp.Y != pos.Y || cp.Z != pos.Z {
		s.Move(pos.X, pos.Y, pos.Z, ns.MoveMode)
	}
	s.SetPosition(pos)
}

// Calls HandlePosition for all positions in the vm.
func HandleAllPositions(s CodeGenerator, m *vm.Machine) {
	for _, x := range m.Positions {
		HandlePosition(s, x)
	}
}
