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

type Streamer struct {
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

func (s *Streamer) Connect(name string) error {
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

func (s *Streamer) Stop() {
	_, _ = s.serialPort.Write([]byte("\x18\n"))
	s.serialPort.Close()
}

func (s *Streamer) Send(doc *gcode.Document, maxPrecision int, progress chan int) error {
	defer close(progress)
	for idx, block := range doc.Blocks {
		_, err := s.writer.WriteString(block.Export(maxPrecision) + "\n")
		if err != nil {
			return errors.New("\nError while sending data:" + fmt.Sprintf("%s", err))
		}
		err = s.writer.Flush()
		if err != nil {
			return errors.New("\nError while flushing writer:" + fmt.Sprintf("%s", err))
		}

		res := serialReader(s.reader)

		switch res.level {
		case "error":
			return errors.New("Received error from CNC: " + res.message)
		case "alarm":
			return errors.New("Received alarm from CNC: " + res.message)
		case "info":
			fmt.Printf("\nReceived info from CNC: %s\n", res.message)
		}
		progress <- idx
	}
	return nil
}
