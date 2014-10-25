package streaming

import "io"
import "github.com/tarm/goserial"
import "github.com/joushou/gocnc/gcode"
import "time"

type Streamer struct {
	serialPort io.ReadWriteCloser
	resultChan chan string
}

func serialReader(s io.ReadWriteCloser, res chan string) {
	buffer := ""
	parseResult := func(b byte) {
		switch b {
		case '\r':
			return
		case '\n':
			s := buffer
			buffer = ""
			if s == "ok" {
				res <- "ok"
			} else if len(s) >= 5 && s[:5] == "error" {
				res <- "error"
			} else if len(s) >= 5 && s[:5] == "ALARM" {
				res <- "error"
			}
		default:
			buffer += string(b)
		}
	}

	b := make([]byte, 1)
	for {
		n, err := s.Read(b)
		if n == 1 {
			parseResult(b[0])
		}
		if err != nil {
			return
		}
	}
}

func waitFor(s io.ReadWriteCloser, w string, max int) bool {
	x := ""
	bytes := 0
	b := make([]byte, 1)
	for {
		n, err := s.Read(b)
		if n == 1 {
			x += string(b[0])
			bytes += 1
			if len(x) > len(w) {
				x = x[len(x)-len(w):]
			}
			if x == w {
				return true
			} else if max != -1 && bytes > max {
				return false
			}
		}
		if err != nil {
			return false
		}
	}
}

func (s *Streamer) Connect(name string) {
	c := &serial.Config{Name: name, Baud: 115200}
	var err error
	s.serialPort, err = serial.OpenPort(c)
	if err != nil {
		panic("Unable to connect to CNC!")
	}

	if !waitFor(s.serialPort, "\r\n", 2) {
		panic("Could not detect initialized GRBL!")
	} else if !waitFor(s.serialPort, "\n", -1) {
		panic("Could not detect initialized GRBL!")
	}

	s.resultChan = make(chan string, 0)
}

func (s *Streamer) Stop() {
	_, _ = s.serialPort.Write([]byte("!\n"))
	s.serialPort.Close()
	close(s.resultChan)
}

func (s *Streamer) Send(doc *gcode.Document, maxPrecision int, progress chan int) {
	go serialReader(s.serialPort, s.resultChan)
	time.Sleep(1 * time.Second)
	for idx, block := range doc.Blocks {
		e := []byte(block.Export(maxPrecision) + "\n")
		n, err := s.serialPort.Write(e)
		if err != nil {
			panic("Unable to write to CNC!")
		}
		if n != len(e) {
			panic("Unable to write all data!")
		}
		res := <-s.resultChan
		if res != "ok" {
			panic("CNC said: " + res)
		}
		progress <- idx
	}
	close(progress)
}
