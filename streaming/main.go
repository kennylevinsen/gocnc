package streaming

import "github.com/joushou/gocnc/vm"

type Streamer interface {
	Check(*vm.Machine) error
	Connect(string, int) error
	Stop()
	Start()
	Pause()
	Send(*vm.Machine, int, chan int) error
}
