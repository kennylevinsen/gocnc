package streaming

import "io"
import "bufio"
import "github.com/joushou/goserial"
import "github.com/joushou/gocnc/vm"
import "github.com/joushou/gocnc/export"
import "errors"
import "fmt"

// A result struct used by serialReader
type result struct {
	level   string
	message string
}

type GrblStreamer struct {
	serialPort io.ReadWriteCloser
	reader     *bufio.Reader
	writer     *bufio.Writer
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
	export.HandleAllPositions(&gen, m)
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

// Sends the vm states. The progress channel sends the current position number as progress info.
func (s *GrblStreamer) Send(m *vm.Machine, maxPrecision int, progress chan int) (err error) {
	defer func() {
		close(progress)
		if r := recover(); r != nil {
			err = errors.New(fmt.Sprintf("\n%s", r))
		}
	}()

	var length, okCnt int
	list := make([]interface{}, 0)
	gen := export.GrblGenerator{}
	gen.Precision = maxPrecision
	gen.Init()

	// handle results
	gen.HandleRes = func() {
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

	gen.Write = func(str string) {
		str += "\n"
		length += len(str)
		list = append(list, str)

		// If Grbl is full...
		for length > 127 {
			gen.HandleRes()
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
		export.HandlePosition(&gen, pos)
		list = append(list, true)
	}

	for okCnt < len(m.Positions) {
		gen.HandleRes()
	}
	return nil
}
