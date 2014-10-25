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

var (
	device     = flag.String("device", "", "Serial device for CNC control")
	inputFile  = flag.String("input", "", "NC file to process")
	outputFile = flag.String("output", "", "Location to dump processed data")
	optVector  = flag.Bool("optvector", true, "Perform vectorized optimization")
	optLifts   = flag.Bool("optlifts", true, "Use rapid position for Z-only upwards moves")
	optDrills  = flag.Bool("optdrill", true, "Use rapid position for drills to last drilled depth")
	precision  = flag.Int("precision", 5, "Precision to use for exported gcode")
)

func init() {
	flag.Parse()
}

func main() {
	if *inputFile == "" {
		fmt.Printf("No file provided\n")
		os.Exit(1)
	}

	if *outputFile == "" && *device == "" {
		fmt.Printf("Requires either device or output file\n")
		os.Exit(2)
	}

	fhandle, err := ioutil.ReadFile(*inputFile)
	if err != nil {
		fmt.Printf("Could not open file: %s\n", err)
		os.Exit(3)
	}

	// Parse
	code := string(fhandle)
	document := gcode.Parse(code)

	// Run through the VM
	var m vm.Machine
	m.Init()

	m.Process(document)

	// Optimize as requested
	if *optVector {
		m.OptimizeMoves()
	}
	if *optLifts {
		m.OptimizeLifts()
	}
	if *optDrills {
		m.OptimizeDrills()
	}

	output := m.Export()

	if *outputFile != "" {
		if err := ioutil.WriteFile(*outputFile, []byte(output.Export(*precision)), 0644); err != nil {
			fmt.Printf("Could not write to file: %s\n", err)
			os.Exit(4)
		}
	}

	if *device != "" {
		var s streaming.Streamer
		if err := s.Connect(*device); err != nil {
			fmt.Printf("Unable to connect to device: %s\n", err)
			os.Exit(5)
		}

		pBar := pb.StartNew(output.Length())
		pBar.Format("[=> ]")

		progress := make(chan int, 0)
		sigchan := make(chan os.Signal, 1)
		signal.Notify(sigchan, os.Interrupt)

		go func() {
			for sig := range sigchan {
				if sig == os.Interrupt {
					fmt.Printf("\nStopping...\n")
					close(progress)
					s.Stop()
					os.Exit(6)
				}
			}
		}()

		go s.Send(output, *precision, progress)
		for _ = range progress {
			pBar.Increment()
		}
		pBar.Finish()
	}

}
