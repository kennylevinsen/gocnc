package streaming

import "github.com/joushou/gocnc/gcode"

type Streamer interface {
	Connect(string) error
	Stop()
	Check(*gcode.Document) error
	Send(*gcode.Document, int, chan int) error
}
