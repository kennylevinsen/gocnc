package streaming

import "github.com/joushou/gocnc/vm"

type Streamer interface {
	Check(*vm.Machine) error
	Connect(string, int) error
	Stop()
	Start()
	Pause()
}
