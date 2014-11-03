package streaming

import "io"
import "bufio"
import "github.com/joushou/goserial"
import "github.com/joushou/gocnc/gcode"
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

func (s *GrblStreamer) Check(doc *gcode.Document) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New(fmt.Sprintf("%s", r))
		}
	}()

	for _, block := range doc.Blocks {
		switch block.Description {
		case "cutter-compensation-set":
			panic("Grbl does not support cutter compensation")
		case "mist-coolant":
			panic("Grbl does not support mist coolant")
		}
	}
	return nil
}

func (s *GrblStreamer) Send(doc *gcode.Document, maxPrecision int, progress chan int) (err error) {
	defer func() {
		close(progress)
		if r := recover(); r != nil {
			err = errors.New(fmt.Sprintf("%s", r))
		}
	}()

	var length, okCnt int
	list := make([]string, 0)

	// handle results
	handleRes := func(res Result) {
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
			length -= len(x)
			progress <- okCnt
			okCnt++
		}
	}

	for _, block := range doc.Blocks {
		x := block.Export(maxPrecision) + "\n"
		length += len(x)
		list = append(list, x)

		// If we need to hack around something...
		switch block.Description {
		case "comment":
			handleRes(Result{"ok", ""})
			continue
		case "cutter-compensation-reset":
			handleRes(Result{"ok", ""})
			continue
		case "tool-change":
			handleRes(Result{"ok", ""})
			continue
		case "cutter-compensation-set":
			panic("Grbl does not support cutter compensation")
		case "mist-coolant":
			panic("Grbl does not support mist coolant")
		}

		// If Grbl is full...
		for length > 127 {
			handleRes(serialReader(s.reader))
		}

		_, err := s.writer.WriteString(x)
		if err != nil {
			return errors.New("\nError while sending data:" + fmt.Sprintf("%s", err))
		}
		err = s.writer.Flush()
		if err != nil {
			return errors.New("\nError while flushing writer:" + fmt.Sprintf("%s", err))
		}
	}

	for okCnt < len(doc.Blocks) {
		handleRes(serialReader(s.reader))
	}

	return nil
}
