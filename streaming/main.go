package streaming

import "io"
import "github.com/tarm/goserial"
import "github.com/joushou/gocnc/gcode"
import "time"

type Streamer struct {
	serialPort io.ReadWriteCloser
}

func (s *Streamer) serialReader(res chan string) {
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

	for {
		b := make([]byte, 1)
		n, err := s.serialPort.Read(b)
		if n == 1 {
			parseResult(b[0])
		}
		if err != nil {
			return
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

	buf := ""
	b := make([]byte, 1)
	_, err = s.serialPort.Read(b)
	if err != nil {
		panic("Unable!")
	}
	buf += string(b)
	_, err = s.serialPort.Read(b)
	if err != nil {
		panic("Unable!!")
	}
	buf += string(b)
	if buf != "\r\n" {
		panic("Unable!!!")
	}
	for {
		_, err = s.serialPort.Read(b)
		if err != nil {
			panic("Unable!!!!")
		}
		if string(b) == "\n" {
			break
		}
	}
}

func (s *Streamer) Stop() {
	_, _ = s.serialPort.Write([]byte("!\n"))
}

func (s *Streamer) Send(doc *gcode.Document, maxPrecision int, progress chan int) {
	ok := make(chan string, 0)
	go s.serialReader(ok)
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
		res := <-ok
		if res != "ok" {
			panic("CNC said: " + res)
		}
		progress <- idx
	}
	close(progress)
}
