package export

import "github.com/joushou/gocnc/vm"
import "strconv"
import "strings"
import "errors"
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

// Interface for exporting a vm position stack.
type CodeGenerator interface {
	GetPosition() vm.Position
	SetPosition(vm.Position)
	ToolChange(int)
	ToolLengthChange(int)
	Spindle(bool, bool, float64)
	Coolant(bool, bool)
	FeedMode(int)
	Feedrate(float64)
	CutterCompensation(int)
	Dwell(float64)
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

// Dummy implementation
func (s *BaseGenerator) ToolChange(int) {
}

// Dummy implementation
func (s *BaseGenerator) ToolLengthChange(int) {
}

// Dummy implementation
func (s *BaseGenerator) Spindle(bool, bool, float64) {
}

// Dummy implementation
func (s *BaseGenerator) Coolant(bool, bool) {
}

// Dummy implementation
func (s *BaseGenerator) FeedMode(int) {
}

// Dummy implementation
func (s *BaseGenerator) Feedrate(float64) {
}

// Dummy implementation
func (s *BaseGenerator) CutterCompensation(int) {
}

// Dummy implementation
func (s *BaseGenerator) Dwell(float64) {
}

// Dummy implementation
func (s *BaseGenerator) Move(float64, float64, float64, int) {
}

// Initializes the current position.
func (s *BaseGenerator) Init() {
	s.Position = vm.Position{State: vm.NewState()}
}

// Calls the CodeGenerator for all changed states.
func HandlePosition(pos vm.Position, gens ...CodeGenerator) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New(fmt.Sprintf("%s", r))
		}
	}()
	for _, s := range gens {
		cp := s.GetPosition()
		cs := cp.State
		ns := pos.State

		if ns.ToolIndex != cs.ToolIndex {
			s.ToolChange(ns.ToolIndex)
		}

		if ns.ToolLengthIndex != cs.ToolLengthIndex {
			s.ToolLengthChange(ns.ToolLengthIndex)
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

		if ns.MoveMode == vm.MoveModeDwell {
			s.Dwell(ns.DwellTime)
		} else if cp.X != pos.X || cp.Y != pos.Y || cp.Z != pos.Z || cs.MoveMode != ns.MoveMode {
			s.Move(pos.X, pos.Y, pos.Z, ns.MoveMode)
		}
		s.SetPosition(pos)
	}
	return nil
}

// Calls HandlePosition for all positions in the vm.
func HandleAllPositions(m *vm.Machine, gens ...CodeGenerator) error {
	for _, x := range m.Positions {
		if err := HandlePosition(x, gens...); err != nil {
			return err
		}
	}
	return nil
}

// Calls HandlePosition for all generators at an index in the vm
func HandlePositionAtIndex(m *vm.Machine, idx int, gens ...CodeGenerator) error {
	for _, x := range gens {
		if err := HandlePosition(m.Positions[idx], x); err != nil {
			return err
		}
	}
	return nil
}
