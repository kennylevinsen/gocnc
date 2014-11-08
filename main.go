package main

import "github.com/joushou/gocnc/gcode"
import "github.com/joushou/gocnc/vm"
import "github.com/joushou/gocnc/export"
import "github.com/joushou/gocnc/streaming"
import "github.com/joushou/pb"

import "io/ioutil"
import "bufio"
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
	stats            = flag.Bool("stats", true, "Print gcode information")
	autoStart        = flag.Bool("autostart", false, "Start sending code without asking questions")
	noOpt            = flag.Bool("noopt", false, "Disable all optimization")
	optBogusMove     = flag.Bool("optbogus", true, "Remove bogus moves")
	optLiftSpeed     = flag.Bool("optlifts", true, "Use rapid position for Z-only upwards moves")
	optDrillSpeed    = flag.Bool("optdrill", true, "Use rapid position for drills to last drilled depth")
	optRouteGrouping = flag.Bool("optroute", true, "Optimize path to groups of routing moves")
	precision        = flag.Int("precision", 4, "Precision to use for exported gcode (max mantissa digits)")
	maxArcDeviation  = flag.Float64("maxarcdeviation", 0.002, "Maximum deviation from an ideal arc (mm)")
	minArcLineLength = flag.Float64("minarclinelength", 0.01, "Minimum arc segment line length (mm)")
	tolerance        = flag.Float64("tolerance", 0.001, "Tolerance used by some position comparisons (mm)")
	feedLimit        = flag.Float64("feedlimit", 0, "Maximum feedrate (mm/min, <= 0 to disable)")
	multiplyFeed     = flag.Float64("multiplyfeed", 0, "Feedrate multiplier (0 to disable)")
	multiplyMove     = flag.Float64("multiplymove", 0, "Move distance multiplier (0 to disable)")
	spindleCW        = flag.Float64("spindlecw", 0, "Force clockwise spindle speed (RPM, <= 0 to disable)")
	spindleCCW       = flag.Float64("spindleccw", 0, "Force counter clockwise spindle speed (RPM, <= 0 to disable)")
	safetyHeight     = flag.Float64("safetyheight", 0, "Enforce safety height (mm, <= 0 to disable)")
	enforceReturn    = flag.Bool("enforcereturn", true, "Enforce rapid return to X0 Y0 Z0")
	flipXY           = flag.Bool("flipxy", false, "Flips the X and Y axes for all moves")
)

func printStats(m *vm.Machine) {
	minx, miny, minz, maxx, maxy, maxz, feedrates := m.Info()
	fmt.Fprintf(os.Stderr, "Metrics\n")
	fmt.Fprintf(os.Stderr, "-------------------------\n")
	fmt.Fprintf(os.Stderr, "   Moves: %d\n", len(m.Positions))
	fmt.Fprintf(os.Stderr, "   Feedrates (mm/min): ")

	for idx, feed := range feedrates {
		if feed == 0 {
			continue
		}
		fmt.Fprintf(os.Stderr, "%g", feed)
		if idx != len(feedrates)-1 {
			fmt.Fprintf(os.Stderr, ", ")
		}
	}
	fmt.Fprintf(os.Stderr, "\n")
	eta := m.ETA()
	meta := (eta / time.Second) * time.Second
	fmt.Fprintf(os.Stderr, "   ETA: %s\n", meta.String())
	fmt.Fprintf(os.Stderr, "   X (mm): %g <-> %g\n", minx, maxx)
	fmt.Fprintf(os.Stderr, "   Y (mm): %g <-> %g\n", miny, maxy)
	fmt.Fprintf(os.Stderr, "   Z (mm): %g <-> %g\n", minz, maxz)
	fmt.Fprintf(os.Stderr, "-------------------------\n")

}

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

	if *spindleCW != 0 && *spindleCCW != 0 {
		fmt.Fprintf(os.Stderr, "Error: Cannot force both clockwise and counter clockwise rotation\n")
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
	m.Init()
	m.MaxArcDeviation = *maxArcDeviation
	m.MinArcLineLength = *minArcLineLength
	m.Tolerance = *tolerance

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
	if *flipXY {
		m.FlipXY()
	}

	if *safetyHeight > 0 {
		if err := m.SetSafetyHeight(*safetyHeight); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not set safety height%s\n", err)
		}
	}

	if *feedLimit > 0 {
		m.LimitFeedrate(*feedLimit)
	}

	if *multiplyFeed != 0 {
		m.FeedrateMultiplier(*multiplyFeed)
	}

	if *multiplyMove != 0 {
		m.MoveMultiplier(*multiplyMove)
	}

	if *spindleCW > 0 {
		m.EnforceSpindle(true, true, *spindleCW)
	} else if *spindleCCW > 0 {
		m.EnforceSpindle(true, false, *spindleCCW)
	}

	if *enforceReturn {
		m.Return()
	}

	if *stats {
		printStats(&m)
	}

	// Handle VM output
	if *debugDump {
		m.Dump()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Could not export vm state: %s\n", err)
		os.Exit(3)
	}

	if *dumpStdout {
		g := export.StringCodeGenerator{Precision: *precision}
		g.Init()
		export.HandleAllPositions(&g, &m)
		fmt.Printf(g.Retrieve())
	}

	if *outputFile != "" {
		g := export.StringCodeGenerator{Precision: *precision}
		g.Init()
		export.HandleAllPositions(&g, &m)

		if err := ioutil.WriteFile(*outputFile, []byte(g.Retrieve()), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Could not write to file: %s\n", err)
			os.Exit(2)
		}
	}

	if *device != "" {
		var s streaming.Streamer = &streaming.GrblStreamer{}

		if err := s.Check(&m); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Incompatibility: %s\n", err)
		}

		if !*autoStart {
			reader := bufio.NewReader(os.Stdin)
			fmt.Fprintf(os.Stderr, "Run code? (y/n) ")
			text, _ := reader.ReadString('\n')
			if text != "y\n" {
				fmt.Fprintf(os.Stderr, "Aborting\n")
				os.Exit(5)
			}
		}

		if err := s.Connect(*device); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Unable to connect to device: %s\n", err)
			os.Exit(2)
		}

		pBar := pb.New(len(m.Positions))
		pBar.Format("[=> ]")
		pBar.Start()

		progress := make(chan int, 0)
		sigchan := make(chan os.Signal, 1)
		signal.Notify(sigchan, os.Interrupt)

		go func() {
			for sig := range sigchan {
				if sig == os.Interrupt {
					fmt.Fprintf(os.Stderr, "\nStopping...\n")
					close(progress)
					s.Stop()
					os.Exit(5)
				}
			}
		}()

		go func() {
			err := s.Send(&m, *precision, progress)
			if err != nil {
				defer func() {
					if r := recover(); r != nil {
						fmt.Fprintf(os.Stderr, "Panic: %s\n", r)
					}
					s.Stop()
					os.Exit(2)
				}()
				fmt.Fprintf(os.Stderr, "\nSend failed: %s\n", err)
				close(progress)
			}
		}()
		for _ = range progress {
			pBar.Increment()
		}
		pBar.Finish()
	}

}
