package streaming

import "io"
import "bufio"
import "github.com/joushou/goserial"
import "github.com/joushou/gocnc/vm"
import "errors"
import "fmt"

type Result struct {
	level   string
	message string
}

type GrblStreamer struct {
	serialPort io.ReadWriteCloser
	reader     *bufio.Reader
	writer     *bufio.Writer
}

type GrblGenerator struct {
	StandardGenerator
}

func (s *GrblGenerator) Toolchange(t int) string {
	defer func() {
		s.state.Tool = t
	}()
	// TODO Implement manual tool-change
	return ""
}

func (s *GrblGenerator) CutterCompensation(cutComp int) string {
	defer func() {
		s.state.CutterCompensation = cutComp
	}()

	if cutComp != vm.CutCompModeNone {
		panic("Cutter compensation not supported by Grbl")
	}
	return ""
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
			err = errors.New(fmt.Sprintf("\n%s", r))
		}
	}()

	var length, okCnt int
	list := make([]interface{}, 0)
	gen := GrblGenerator{}
	gen.Precision = maxPrecision
	gen.Init()

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
			panic(fmt.Sprintf("Received error from CNC: %s, block: %s", res.message, list[0]))
		case "alarm":
			panic(fmt.Sprintf("Received alarm from CNC: %s, block: %s", res.message, list[0]))
		case "info":
			fmt.Printf("\nReceived info from CNC: %s\n", res.message)
		default:
			x := list[0]
			list = list[1:]
			if i, ok := x.(string); ok {
				length -= len(i)
			}
		}
	}

	write := func(str string) {
		length += len(str)
		list = append(list, str)

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
		x := HandlePosition(&gen, pos)
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
