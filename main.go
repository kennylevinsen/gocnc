package main

import "github.com/joushou/gocnc/gcode"
import "github.com/joushou/gocnc/vm"
import "github.com/joushou/gocnc/streaming"
import "github.com/cheggaaa/pb"

import "io/ioutil"
import "flag"
import "fmt"
import "os"
import "os/signal"

func main() {
	flag.Parse()
	b, err := ioutil.ReadFile(flag.Args()[0])
	if err != nil {
		fmt.Printf("No such file: %s\n", err)
	}
	test := string(b)
	doc := gcode.Parse(test)
	var m vm.Machine
	m.Init()
	m.Process(doc)
	m.OptimizeMoves()
	m.OptimizeLifts()
	m.OptimizeDrills()
	doc = m.Export()

	var s streaming.Streamer

	s.Connect(flag.Args()[1])

	pBar := pb.StartNew(doc.Length())
	pBar.Format("[=> ]")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	progress := make(chan int, 0)
	go func() {
		for _ = range c {
			fmt.Printf("\n<C-c> Stopping\n")
			close(progress)
			s.Stop()
			os.Exit(1)
		}
	}()

	go s.Send(doc, 4, progress)
	for _ = range progress {
		pBar.Increment()
	}
	pBar.Finish()
}
