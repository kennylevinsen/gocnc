package main

import "github.com/joushou/gocnc/gcode"
import "github.com/joushou/gocnc/vm"
import "github.com/joushou/gocnc/export"
import "github.com/joushou/gocnc/streaming"
import "github.com/joushou/pb"
import "gopkg.in/alecthomas/kingpin.v1"

import "io/ioutil"
import "bufio"

import "fmt"
import "os"
import "os/signal"
import "time"
import "strconv"

var (
	inputFile  = kingpin.Arg("input", "Input file").Required().ExistingFile()
	device     = kingpin.Flag("device", "Serial device for gcode").Short('d').ExistingFile()
	baudrate   = kingpin.Flag("baudrate", "Baudrate for serial device").Short('b').Default("115200").Int()
	outputFile = kingpin.Flag("output", "Output file for gcode").Short('o').String()

	dumpStdout = kingpin.Flag("stdout", "Dump gcode to stdout").Bool()
	debugDump  = kingpin.Flag("debugdump", "Dump VM state to stdout").Hidden().Bool()

	stats     = kingpin.Flag("stats", "Print gcode metrics").Default("true").Bool()
	autoStart = kingpin.Flag("autostart", "Start sending code without asking questions").Bool()

	opt              = kingpin.Flag("opt", "Allow optimizations").Default("true").Bool()
	optBogusMove     = kingpin.Flag("optbogus", "Remove bogus moves").Default("true").Bool()
	optLiftSpeed     = kingpin.Flag("optlifts", "Use rapid positioning for Z-only upwards moves").Default("true").Bool()
	optDrillSpeed    = kingpin.Flag("optdrill", "Use rapid positioning for drills to last drilled depth").Default("true").Bool()
	optRouteGrouping = kingpin.Flag("optroute", "Optimize path to groups of routing moves").Default("true").Bool()

	precision        = kingpin.Flag("precision", "Precision to use for exported gcode (max mantissa digits)").Default("4").Int()
	maxArcDeviation  = kingpin.Flag("maxarcdeviation", "Maximum deviation from an ideal arc (mm)").Default("0.002").Float()
	minArcLineLength = kingpin.Flag("minarclinelength", "Minimum arc segment line length (mm)").Default("0.01").Float()
	tolerance        = kingpin.Flag("tolerance", "Tolerance used by some position comparisons (mm)").Default("0.001").Float()

	feedLimit    = kingpin.Flag("feedlimit", "Maximum feedrate (mm/min, <= 0 to disable)").Float()
	safetyHeight = kingpin.Flag("safetyheight", "Enforce safety height (mm, <= 0 to disable)").Float()
	multiplyFeed = kingpin.Flag("multiplyfeed", "Feedrate multiplier (0 to disable)").Float()
	multiplyMove = kingpin.Flag("multiplymove", "Move distance multiplier (0 to disable)").Float()

	spindleCW  = kingpin.Flag("spindlecw", "Force clockwise spindle speed (RPM, <= 0 to disable)").Float()
	spindleCCW = kingpin.Flag("spindleccw", "Force counter clockwise spindle speed (RPM, <= 0 to disable)").Float()

	enforceReturn    = kingpin.Flag("enforcereturn", "Enforce rapid return to X0 Y0 Z0").Default("true").Bool()
	flipXY           = kingpin.Flag("flipxy", "Flips the X and Y axes for all moves").Bool()
	manualToolchange = kingpin.Flag("manualtoolchange", "Wait for manual toolchange on M6").Bool()
	toolchangeHeight = kingpin.Flag("toolchangeheight", "Height to go to for toolchange (0 to use safety height)").Default("0").Float()
)

type ManualToolchange struct {
	export.BaseGenerator
	machine    *vm.Machine
	gen        export.CodeGenerator
	toolLength float64
	hasChanged bool
}

func (m *ManualToolchange) Toolchange(i int) {
	if !*manualToolchange {
		return
	}

	// Go to X0Y0 and Z as requested
	newHeight := 0.0
	if *toolchangeHeight == 0 {
		newHeight = m.machine.FindSafetyHeight()
	} else {
		newHeight = *toolchangeHeight
	}

	if m.hasChanged {
		curPos := m.GetPosition()
		newPos := curPos
		newPos.Z = newHeight
		export.HandlePosition(newPos, m.gen)
		newPos.X = 0
		newPos.Y = 0
		newPos.State.SpindleEnabled = false
		newPos.State.MistCoolant = false
		newPos.State.FloodCoolant = false
		export.HandlePosition(newPos, m.gen)
	}

	// Await tool info
	reader := bufio.NewReader(os.Stdin)
	toolLength := m.toolLength
	for {
		if !m.hasChanged {
			fmt.Fprintf(os.Stderr, "Change to tool %d. New tool length (First tool, will not change offset) [%f]: ", i, toolLength)
		} else {
			fmt.Fprintf(os.Stderr, "Change to tool %d. New tool length [%f]: ", i, toolLength)
		}
		text, _ := reader.ReadString('\n')
		if len(text) == 0 {
			panic("No data from os.stdin")
		}
		text = text[:len(text)-1]
		if text == "" {
			break
		} else if t, err := strconv.ParseFloat(text, 64); err == nil {
			toolLength = t
			break
		}
	}

	if m.hasChanged {
		change := toolLength - m.toolLength

		for idx, _ := range m.machine.Positions {
			m.machine.Positions[idx].Z += change
		}

		curPos := m.GetPosition()
		newPos := curPos
		newPos.Z = newHeight

		export.HandlePosition(newPos, m.gen)
		newPos.Z = curPos.Z + change
		export.HandlePosition(newPos, m.gen)
	}

	m.toolLength = toolLength
	m.hasChanged = true

}

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
	kingpin.Parse()

	if *spindleCW != 0 && *spindleCCW != 0 {
		fmt.Fprintf(os.Stderr, "Error: Cannot force both clockwise and counter clockwise rotation\n")
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
	if *optDrillSpeed && *opt {
		m.OptDrillSpeed()
	}

	if *optRouteGrouping && *opt {
		if err := m.OptRouteGrouping(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not execute route grouping: %s\n", err)
		}
	}

	if *optBogusMove && *opt {
		m.OptBogusMoves()
	}

	if *optLiftSpeed && *opt {
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
		export.HandleAllPositions(&m, &g)
		fmt.Printf(g.Retrieve())
	}

	if *outputFile != "" {
		g := export.StringCodeGenerator{Precision: *precision}
		g.Init()
		export.HandleAllPositions(&m, &g)

		if err := ioutil.WriteFile(*outputFile, []byte(g.Retrieve()), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Could not write to file: %s\n", err)
			os.Exit(2)
		}
	}

	if *device != "" {
		s := &streaming.GrblStreamer{}
		s.Precision = *precision
		mt := &ManualToolchange{machine: &m, gen: s}

		s.Init()
		mt.Init()

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

		if err := s.Connect(*device, *baudrate); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Unable to connect to device: %s\n", err)
			os.Exit(2)
		}

		pBar := pb.New(len(m.Positions))
		pBar.ManualUpdate = true
		pBar.Format("[=> ]")
		pBar.Start()

		sigchan := make(chan os.Signal, 1)
		signal.Notify(sigchan, os.Interrupt)

		go func() {
			for sig := range sigchan {
				if sig == os.Interrupt {
					fmt.Fprintf(os.Stderr, "\nStopping...\n")
					s.Stop()
					os.Exit(5)
				}
			}
		}()

		for idx, _ := range m.Positions {
			export.HandlePosition(m.Positions[idx], mt)
			if err := export.HandlePosition(m.Positions[idx], s); err != nil {
				s.Stop()
				panic(err)
			}
			pBar.Increment()
			pBar.Update()
		}
		pBar.Finish()
		pBar.Update()
	}

}
