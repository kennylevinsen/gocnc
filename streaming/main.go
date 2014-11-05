package streaming

import "github.com/joushou/gocnc/vm"

type Streamer interface {
	Connect(string) error
	Stop()
	Send(*vm.Machine, int, chan int) error
}
