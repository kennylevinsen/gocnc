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
import "time"

var (
	device        = flag.String("device", "", "Serial device for CNC control")
	inputFile     = flag.String("input", "", "NC file to process")
	outputFile    = flag.String("output", "", "Location to dump processed data")
	dumpStdout    = flag.Bool("stdout", false, "Output to stdout")
	optVector     = flag.Bool("optvector", true, "Perform vectorized optimization")
	optLifts      = flag.Bool("optlifts", true, "Use rapid position for Z-only upwards moves")
	optDrills     = flag.Bool("optdrill", true, "Use rapid position for drills to last drilled depth")
	precision     = flag.Int("precision", 5, "Precision to use for exported gcode")
	feedLimit     = flag.Float64("feedlimit", -1, "Maximum feedrate")
	safetyHeight  = flag.Float64("safetyheight", -1, "Enforce safety height")
	enforceReturn = flag.Bool("enforcereturn", true, "Enforce rapid return to X0 Y0 Z0")
)

func main() {
	// Parse arguments
	flag.Parse()
	if len(flag.Args()) > 0 {
		flag.Usage()
		os.Exit(1)
	}

	if *inputFile == "" {
		fmt.Printf("Error: No file provided\n")
		os.Exit(1)
	}

	if *outputFile == "" && *device == "" && !*dumpStdout {
		fmt.Printf("Error: No output location provided\n")
		os.Exit(1)
	}

	fhandle, err := ioutil.ReadFile(*inputFile)
	if err != nil {
		fmt.Printf("Error: Could not open file: %s\n", err)
		os.Exit(2)
	}

	// Parse
	code := string(fhandle)
	document := gcode.Parse(code)

	// Run through the VM
	var m vm.Machine
	m.Init()

	m.Process(document)

	// Apply requested modifications
	if *enforceReturn {
		m.Return()
	}
	if *safetyHeight > 0 {
		err := m.SetSafetyHeight(*safetyHeight)
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			os.Exit(3)
		}
	}

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
	if *feedLimit > 0 {
		m.LimitFeedrate(*feedLimit)
	}
	output := m.Export()

	if *dumpStdout {
		fmt.Printf(output.Export(*precision) + "\n")
	}

	if *outputFile != "" {
		if err := ioutil.WriteFile(*outputFile, []byte(output.Export(*precision)), 0644); err != nil {
			fmt.Printf("Error: Could not write to file: %s\n", err)
			os.Exit(2)
		}
	}

	if *device != "" {
		startTime := time.Now()
		var s streaming.Streamer
		if err := s.Connect(*device); err != nil {
			fmt.Printf("Error: Unable to connect to device: %s\n", err)
			os.Exit(2)
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
					os.Exit(7)
				}
			}
		}()

		go func() {
			err := s.Send(output, *precision, progress)
			if err != nil {
				fmt.Printf("\nSend failed: %s\n", err)
				close(progress)
				s.Stop()
				os.Exit(2)
			}
		}()
		for _ = range progress {
			pBar.Increment()
		}
		pBar.Finish()
		fmt.Printf("%s\n", time.Now().Sub(startTime).String())
	}

}
