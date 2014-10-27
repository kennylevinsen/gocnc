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
import "net/http"
import _ "net/http/pprof"

var (
	device       = flag.String("device", "", "Serial device for CNC control")
	inputFile    = flag.String("input", "", "NC file to process")
	outputFile   = flag.String("output", "", "Location to dump processed data")
	optVector    = flag.Bool("optvector", true, "Perform vectorized optimization")
	optLifts     = flag.Bool("optlifts", true, "Use rapid position for Z-only upwards moves")
	optDrills    = flag.Bool("optdrill", true, "Use rapid position for drills to last drilled depth")
	precision    = flag.Int("precision", 5, "Precision to use for exported gcode")
	feedLimit    = flag.Float64("feedlimit", -1, "Maximum feedrate")
	safetyHeight = flag.Float64("safetyheight", -1, "Enforce safety height")
	ensureReturn = flag.Bool("enforcereturn", true, "Enforce rapid return to X0 Y0 Z0")
)

func init() {
	go func() {
		// Debugging assistance
		fmt.Printf("\n%s\n", http.ListenAndServe("localhost:6060", nil))
	}()
	flag.Parse()
}

func main() {
	if *inputFile == "" {
		fmt.Printf("Error: No file provided\n")
		os.Exit(1)
	}

	if *outputFile == "" && *device == "" {
		fmt.Printf("Error: Requires either device or output file\n")
		os.Exit(2)
	}

	fhandle, err := ioutil.ReadFile(*inputFile)
	if err != nil {
		fmt.Printf("Error: Could not open file: %s\n", err)
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
	if *feedLimit > 0 {
		m.LimitFeedrate(*feedLimit)
	}
	if *safetyHeight > 0 {
		err := m.SetSafetyHeight(*safetyHeight)
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			os.Exit(4)
		}
	}
	if *ensureReturn {
		m.Return()
	}
	output := m.Export()

	if *outputFile != "" {
		if err := ioutil.WriteFile(*outputFile, []byte(output.Export(*precision)), 0644); err != nil {
			fmt.Printf("Error: Could not write to file: %s\n", err)
			os.Exit(5)
		}
	}

	if *device != "" {
		var s streaming.Streamer
		if err := s.Connect(*device); err != nil {
			fmt.Printf("Error: Unable to connect to device: %s\n", err)
			os.Exit(6)
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
				os.Exit(8)
			}
		}()
		for _ = range progress {
			pBar.Increment()
		}
		pBar.Finish()
	}

}
