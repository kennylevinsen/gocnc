package streaming

import "io"
import "bufio"
import "github.com/kennylevinsen/goserial"
import "github.com/kennylevinsen/gocnc/vm"
import "github.com/kennylevinsen/gocnc/export"
import "errors"
import "fmt"

// A result struct used by serialReader
type result struct {
	level   string
	message string
}

type GrblStreamer struct {
	export.GrblGenerator
	serialPort io.ReadWriteCloser
	reader     *bufio.Reader
	writer     *bufio.Writer
	generator  *export.GrblGenerator
}

//
// Serial handling
//

// Awaits and reads a response from Grbl
func serialReader(reader *bufio.Reader) result {
	c, err := reader.ReadBytes('\n')
	if err != nil {
		return result{"serial-error", fmt.Sprintf("%s", err)}
	}
	b := string(c)
	if b == "ok\r\n" {
		return result{"ok", ""}
	} else if len(b) >= 5 && b[:5] == "error" {
		return result{"error", b[6 : len(b)-1]}
	} else if len(b) >= 5 && b[:5] == "alarm" {
		return result{"alarm", b[6 : len(b)-1]}
	} else {
		return result{"info", b[:len(b)-1]}
	}
}

func (s *GrblStreamer) handleRes(str string) {
	// Look for a response
	res := serialReader(s.reader)

	switch res.level {
	case "error":
		panic(fmt.Sprintf("Received error from CNC: %s, block: %s", res.message, str))
	case "alarm":
		panic(fmt.Sprintf("Received alarm from CNC: %s, block: %s", res.message, str))
	case "info":
		fmt.Printf("\nReceived info from CNC: %s\n", res.message)
	default:
	}
}

func (s *GrblStreamer) Init() {
	s.Write = func(str string) {
		str += "\n"

		_, err := s.writer.WriteString(str)
		if err != nil {
			panic(fmt.Sprintf("Error while sending data: %s", err))
		}
		err = s.writer.Flush()
		if err != nil {
			panic(fmt.Sprintf("Error while flushing writer: %s", err))
		}
		s.handleRes(str)
	}
	s.GrblGenerator.Init()
}

// Takes the vm for a dry-run, to see if the states are compatible with Grbl.
func (s *GrblStreamer) Check(m *vm.Machine) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New(fmt.Sprintf("%s", r))
		}
	}()
	gen := export.GrblGenerator{}
	gen.Init()
	gen.Write = func(string) {}
	export.HandleAllPositions(m, &gen)
	return nil
}

// Connect to a serial port at the given path and baudrate
func (s *GrblStreamer) Connect(name string, baud int) error {
	c := &serial.Config{Name: name, Baud: baud}
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

// Raises a position alarm in Grbl. Works as emergency stop.
func (s *GrblStreamer) Stop() {
	_, _ = s.serialPort.Write([]byte("\x18"))
	s.serialPort.Close()
}

// Issues a cycle-start ("~")
func (s *GrblStreamer) Start() {
	_, _ = s.serialPort.Write([]byte("~"))
}

// Issues a feed-hold ("!")
func (s *GrblStreamer) Pause() {
	_, _ = s.serialPort.Write([]byte("!"))
}
