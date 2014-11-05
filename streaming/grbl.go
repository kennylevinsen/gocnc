package streaming

import "io"
import "bufio"
import "github.com/joushou/goserial"
import "github.com/joushou/gocnc/vm"
import "errors"
import "fmt"
import "strconv"
import "strings"

type Result struct {
	level   string
	message string
}

type GrblStreamer struct {
	serialPort          io.ReadWriteCloser
	reader              *bufio.Reader
	writer              *bufio.Writer
	precision           int
	state               vm.State
	lastx, lasty, lastz float64
}

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

//
// Code generator
//

func (s *GrblStreamer) Toolchange(t int) string {
	defer func() {
		s.state.Tool = t
	}()
	if s.state.Tool != t {
		// TODO Implement a manual tool-change handling!
	}
	return ""
}

func (s *GrblStreamer) Spindle(enabled, clockwise bool, speed float64) string {
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
		x += fmt.Sprintf("S%s", floatToString(speed, s.precision))
	}

	return x
}

func (s *GrblStreamer) Coolant(floodCoolant, mistCoolant bool) string {
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

func (s *GrblStreamer) FeedMode(feedMode int) string {
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

func (s *GrblStreamer) Feedrate(feedrate float64) string {
	defer func() {
		s.state.Feedrate = feedrate
	}()

	if s.state.Feedrate == feedrate {
		return ""
	}

	return fmt.Sprintf("F%s", floatToString(feedrate, s.precision))
}

func (s *GrblStreamer) CutterCompensation(cutComp int) string {
	defer func() {
		s.state.CutterCompensation = cutComp
	}()

	if s.state.CutterCompensation == cutComp {
		return ""
	}

	switch cutComp {
	case vm.CutCompModeNone:
		// Emit G40
		return ""
	case vm.CutCompModeOuter:
		// Emit G41
		panic("Cutter compensation not supported by GRBL")
	case vm.CutCompModeInner:
		// Emit G42
		panic("Cutter compensation not supported by GRBL")
	}
	panic("Unknown cutter compensation mode")
}

func (s *GrblStreamer) Move(x, y, z float64, moveMode int) string {
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
		w += fmt.Sprintf("X%s", floatToString(x, s.precision))
	}
	if s.lasty != y {
		w += fmt.Sprintf("Y%s", floatToString(y, s.precision))
	}
	if s.lastz != z {
		w += fmt.Sprintf("Z%s", floatToString(z, s.precision))
	}

	return w
}

func (s *GrblStreamer) HandlePosition(pos vm.Position) []string {
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

//
// Serial handling
//

func serialReader(reader *bufio.Reader) Result {
	c, err := reader.ReadBytes('\n')
	if err != nil {
		return Result{"serial-error", fmt.Sprintf("%s", err)}
	}
	b := string(c)
	if b == "ok\r\n" {
		return Result{"ok", ""}
	} else if len(b) >= 5 && b[:5] == "error" {
		return Result{"error", b[6 : len(b)-1]}
	} else if len(b) >= 5 && b[:5] == "alarm" {
		return Result{"alarm", b[6 : len(b)-1]}
	} else {
		return Result{"info", b[:len(b)-1]}
	}
}

func (s *GrblStreamer) Connect(name string) error {
	c := &serial.Config{Name: name, Baud: 115200}
	var err error
	s.serialPort, err = serial.OpenPort(c)
	if err != nil {
		return err
	}

	s.reader = bufio.NewReader(s.serialPort)
	s.writer = bufio.NewWriter(s.serialPort)

	for {
		c, err := s.reader.ReadBytes('\n')
		m := string(c)
		if len(m) == 26 && m[:5] == "Grbl " && m[9:] == " ['$' for help]\r\n" {
			fmt.Printf("Grbl version %s initialized\n", m[5:9])
			break
		} else if m == "\r\n" {
			continue
		}

		if err != nil {
			return errors.New("Unable to detect initialized GRBL")
		}
	}

	return nil
}

func (s *GrblStreamer) Stop() {
	_, _ = s.serialPort.Write([]byte("\x18\n"))
	s.serialPort.Close()
}

func (s *GrblStreamer) Send(m *vm.Machine, maxPrecision int, progress chan int) (err error) {
	defer func() {
		close(progress)
		if r := recover(); r != nil {
			err = errors.New(fmt.Sprintf("%s", r))
		}
	}()

	s.state = vm.State{0, 0, 0, -1, false, false, false, false, -1, -1}
	s.precision = maxPrecision

	var length, okCnt int
	list := make([]interface{}, 0)

	// handle results
	handleRes := func() {
		if len(list) == 0 {
			return
		}

		// See if we met a checkpoint
		if _, ok := list[0].(bool); ok {
			list = list[1:]
			progress <- okCnt
			okCnt++
			return
		}

		// Look for a response
		res := serialReader(s.reader)

		switch res.level {
		case "error":
			panic("Received error from CNC: " + res.message)
		case "alarm":
			panic("Received alarm from CNC: " + res.message)
		case "info":
			fmt.Printf("\nReceived info from CNC: %s\n", res.message)
		default:
			x := list[0]
			list = list[1:]
			if i, ok := x.(int); ok {
				length -= i
			}
		}
	}

	write := func(str string) {
		length += len(str)
		list = append(list, len(str))

		// If Grbl is full...
		for length > 127 {
			handleRes()
		}

		_, err := s.writer.WriteString(str)
		if err != nil {
			panic(fmt.Sprintf("Error while sending data: %s", err))
		}
		err = s.writer.Flush()
		if err != nil {
			panic(fmt.Sprintf("Error while flushing writer: %s", err))
		}
	}

	for _, pos := range m.Positions {
		x := s.HandlePosition(pos)
		for _, m := range x {
			write(m + "\n")
		}
		list = append(list, true)
	}

	for okCnt < len(m.Positions) {
		handleRes()
	}
	return nil
}
