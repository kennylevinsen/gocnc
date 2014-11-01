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
	device           = flag.String("device", "", "Serial device for CNC control")
	inputFile        = flag.String("input", "", "NC file to process")
	outputFile       = flag.String("output", "", "Location to dump processed data")
	dumpStdout       = flag.Bool("stdout", false, "Output to stdout")
	debugDump        = flag.Bool("debugdump", false, "Dump VM position state after optimization")
	noOpt            = flag.Bool("noopt", false, "Disable all optimization")
	optBogusMove     = flag.Bool("optbogus", true, "Remove bogus moves")
	optLiftSpeed     = flag.Bool("optlifts", true, "Use rapid position for Z-only upwards moves")
	optDrillSpeed    = flag.Bool("optdrill", true, "Use rapid position for drills to last drilled depth")
	optRouteGrouping = flag.Bool("optroute", true, "Optimize path to groups of routing moves")
	precision        = flag.Int("precision", 4, "Precision to use for exported gcode")
	maxArcDeviation  = flag.Float64("maxarcdeviation", 0.002, "Maximum deviation from an ideal arc")
	minArcLineLength = flag.Float64("minarclinelength", 0.01, "Minimum arc segment line length")
	feedLimit        = flag.Float64("feedlimit", -1, "Maximum feedrate")
	safetyHeight     = flag.Float64("safetyheight", -1, "Enforce safety height")
	enforceReturn    = flag.Bool("enforcereturn", true, "Enforce rapid return to X0 Y0 Z0")
)

func main() {
	// Parse arguments
	flag.Parse()
	if len(flag.Args()) > 0 {
		flag.Usage()
		os.Exit(1)
	}

	if *inputFile == "" {
		fmt.Fprintf(os.Stderr, "Error: No file provided\n")
		flag.Usage()
		os.Exit(1)
	}

	if *outputFile == "" && *device == "" && !*dumpStdout && !*debugDump {
		fmt.Fprintf(os.Stderr, "Error: No output location provided\n")
		flag.Usage()
		os.Exit(1)
	}

	fhandle, err := ioutil.ReadFile(*inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Could not open file: %s\n", err)
		os.Exit(2)
	}

	// Parse
	code := string(fhandle)
	document := gcode.Parse(code)

	// Run through the VM
	var m vm.Machine
	m.Init(*maxArcDeviation, *minArcLineLength)

	if err := m.Process(document); err != nil {
		fmt.Fprintf(os.Stderr, "VM failed: %s\n", err)
		os.Exit(3)
	}

	// Optimize as requested
	if *optDrillSpeed && !*noOpt {
		m.OptDrillSpeed()
	}

	if *optRouteGrouping && !*noOpt {
		if err := m.OptRouteGrouping(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not execute route grouping: %s\n", err)
		}
	}

	if *optBogusMove && !*noOpt {
		m.OptBogusMoves()
	}

	if *optLiftSpeed && !*noOpt {
		m.OptLiftSpeed()
	}

	// Apply requested modifications
	if *safetyHeight > 0 {
		err := m.SetSafetyHeight(*safetyHeight)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(3)
		}
	}

	if *feedLimit > 0 {
		m.LimitFeedrate(*feedLimit)
	}

	if *enforceReturn {
		m.Return()
	}

	// Handle VM output
	if *debugDump {
		m.Dump()
	}
	output, err := m.Export()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Could not export vm state: %s\n", err)
		os.Exit(3)
	}

	if *dumpStdout {
		fmt.Printf(output.Export(*precision) + "\n")
	}

	if *outputFile != "" {
		if err := ioutil.WriteFile(*outputFile, []byte(output.Export(*precision)), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Could not write to file: %s\n", err)
			os.Exit(2)
		}
	}

	if *device != "" {
		startTime := time.Now()
		var s streaming.Streamer
		if err := s.Connect(*device); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Unable to connect to device: %s\n", err)
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
					fmt.Fprintf(os.Stderr, "\nStopping...\n")
					close(progress)
					s.Stop()
					os.Exit(7)
				}
			}
		}()

		go func() {
			err := s.Send(output, *precision, progress)
			if err != nil {
				fmt.Fprintf(os.Stderr, "\nSend failed: %s\n", err)
				close(progress)
				s.Stop()
				os.Exit(2)
			}
		}()
		for _ = range progress {
			pBar.Increment()
		}
		pBar.Finish()
		fmt.Fprintf(os.Stderr, "%s\n", time.Now().Sub(startTime).String())
	}

}
