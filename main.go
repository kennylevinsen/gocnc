package main

import "github.com/joushou/gocnc/gcode"
import "github.com/joushou/gocnc/vm"
import "github.com/joushou/gocnc/optimize"
import "github.com/joushou/gocnc/export"
import "github.com/joushou/gocnc/streaming"
import "github.com/cheggaaa/pb"
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
	optBogusMove     = kingpin.Flag("optbogus", "Remove all moves that would be an implicit part of another move (Deprecated for optvector)").Default("false").Bool()
	optVector        = kingpin.Flag("optvector", "Remove all B moves that deviate from the line AC more than tolerance").Default("true").Bool()
	optLiftSpeed     = kingpin.Flag("optlifts", "Use rapid positioning for Z-only upwards moves").Default("true").Bool()
	optDrillSpeed    = kingpin.Flag("optdrill", "Use rapid positioning for drills to last drilled depth").Default("true").Bool()
	optRouteGrouping = kingpin.Flag("optroute", "Optimize path to groups of routing moves").Default("false").Bool()

	precision        = kingpin.Flag("precision", "Precision to use for exported gcode (max mantissa digits)").Default("4").Int()
	maxArcDeviation  = kingpin.Flag("maxarcdeviation", "Maximum deviation from an ideal arc (mm)").Default("0.002").Float()
	minArcLineLength = kingpin.Flag("minarclinelength", "Minimum arc segment line length (mm)").Default("0.01").Float()
	rtolerance       = kingpin.Flag("rtolerance", "Tolerance used by route grouping (mm)").Default("0.001").Float()
	vtolerance       = kingpin.Flag("vtolerance", "Tolerance used by vector optimization (mm)").Default("0.0003").Float()

	feedLimit    = kingpin.Flag("feedlimit", "Maximum feedrate (mm/min, <= 0 to disable)").Float()
	safetyHeight = kingpin.Flag("safetyheight", "Enforce safety height (mm, <= 0 to disable)").Float()
	multiplyFeed = kingpin.Flag("multiplyfeed", "Feedrate multiplier (0 to disable)").Float()
	multiplyMove = kingpin.Flag("multiplymove", "Move distance multiplier (0 to disable)").Float()

	spindleCW  = kingpin.Flag("spindlecw", "Force clockwise spindle speed (RPM, <= 0 to disable)").Float()
	spindleCCW = kingpin.Flag("spindleccw", "Force counter clockwise spindle speed (RPM, <= 0 to disable)").Float()

	enforceReturn    = kingpin.Flag("enforcereturn", "Enforce rapid return to X0 Y0 Z0").Default("true").Bool()
	flipXY           = kingpin.Flag("flipxy", "Flips the X and Y axes for all moves").Bool()
	manualToolchange = kingpin.Flag("manualtool", "Wait for manual toolchange operation").Bool()
	manualSpindle    = kingpin.Flag("manualspindle", "Wait for manual spindle operation").Bool()
	manualCoolant    = kingpin.Flag("manualcoolant", "Wait for manual coolant operation").Bool()
	spindleWait      = kingpin.Flag("spindlewait", "Seconds to dwell after spindle changes").Int()
	coolantWait      = kingpin.Flag("coolantwait", "Seconds to dwell after coolant changes").Int()
	toolchangeHeight = kingpin.Flag("tcheight", "Height to go to for toolchange (0 to use safety height)").Default("0").Float()
)

var (
	generators []export.CodeGenerator
	machine    vm.Machine
)

//
// WaitGenerator
//

type WaitGenerator struct {
	export.BaseGenerator
}

// Waits a certain time after spindle changes
func (m *WaitGenerator) Spindle(bool, bool, float64) {
	if *spindleWait > 0 {
		time.Sleep(time.Duration(*spindleWait) * time.Second)
	}
}

// Waits a certain time after coolant changes
func (m *WaitGenerator) Coolant(bool, bool) {
	if *coolantWait > 0 {
		time.Sleep(time.Duration(*spindleWait) * time.Second)
	}
}

//
// ManualGenerator
//

// A generator implement user interaction
type ManualGenerator struct {
	export.BaseGenerator
	gen        export.CodeGenerator
	toolLength float64
	hasChanged bool
	tguard     int
	sguard     int
}

// Prompts user to make the requested changes to spindle, waits for <ENTER>
func (m *ManualGenerator) Spindle(enabled, clockwise bool, speed float64) {
	if m.sguard > 0 {
		return
	}
	m.sguard++
	defer func() {
		m.sguard--
	}()

	if !*manualSpindle {
		return
	}

	if enabled {
		if clockwise {
			fmt.Fprintf(os.Stderr, "Set spindle to clockwise rotation at %.2f RPM. Confirm with <ENTER>", speed)
		} else {
			fmt.Fprintf(os.Stderr, "Set spindle to counter clockwise rotation at %.2f RPM. Confirm with <ENTER>", speed)
		}
	} else {
		fmt.Fprintf(os.Stderr, "Disable spindle. Confirm with <ENTER>")
	}
	reader := bufio.NewReader(os.Stdin)
	_, _ = reader.ReadString('\n')
}

// Prompts the user to make the request changes to spindle, waits for <ENTER>
func (m *ManualGenerator) Coolant(floodCoolant, mistCoolant bool) {
	if !*manualCoolant {
		return
	}
	if !floodCoolant && !mistCoolant {
		fmt.Fprintf(os.Stderr, "Disable coolant. Confirm with <ENTER>")
	} else if floodCoolant && mistCoolant {
		fmt.Fprintf(os.Stderr, "Enable flood and mist coolant. Confirm with <ENTER>")
	} else if floodCoolant {
		fmt.Fprintf(os.Stderr, "Enable flood coolant. Confirm with <ENTER>")
	} else if mistCoolant {
		fmt.Fprintf(os.Stderr, "Enable mist coolant. Confirm with <ENTER>")
	}
	reader := bufio.NewReader(os.Stdin)
	_, _ = reader.ReadString('\n')
}

// Moves spindle to easily accessible spot, and prompts for toolchange
func (m *ManualGenerator) Toolchange(i int) {
	// Multiple entry guard!
	if m.tguard > 0 {
		return
	}
	m.tguard++
	defer func() {
		m.tguard--
	}()

	if !*manualToolchange {
		return
	}

	// Go to X0Y0 and Z as requested
	newHeight := 0.0
	if *toolchangeHeight == 0 {
		newHeight = machine.FindSafetyHeight()
	} else {
		newHeight = *toolchangeHeight
	}

	curPos := m.GetPosition()

	if m.hasChanged {
		newPos := curPos
		newPos.Z = newHeight
		export.HandlePosition(newPos, generators...)
		newPos.X = 0
		newPos.Y = 0
		newPos.State.SpindleEnabled = false
		newPos.State.MistCoolant = false
		newPos.State.FloodCoolant = false
		export.HandlePosition(newPos, generators...)
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

		for idx, _ := range machine.Positions {
			machine.Positions[idx].Z += change
		}

		newPos := curPos
		newPos.Z = newHeight

		export.HandlePosition(newPos, generators...)
		newPos.Z = curPos.Z + change
		export.HandlePosition(newPos, generators...)
	}

	m.toolLength = toolLength
	m.hasChanged = true
}

func printStats(m *vm.Machine) {
	minx, miny, minz, maxx, maxy, maxz, feedrates := machine.Info()
	fmt.Fprintf(os.Stderr, "Metrics\n")
	fmt.Fprintf(os.Stderr, "-------------------------\n")
	fmt.Fprintf(os.Stderr, "   Moves: %d\n", len(machine.Positions))
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
	eta := machine.ETA()
	meta := (eta / time.Second) * time.Second
	fmt.Fprintf(os.Stderr, "   ETA: %s\n", meta.String())
	fmt.Fprintf(os.Stderr, "   X (mm): %g <-> %g\n", minx, maxx)
	fmt.Fprintf(os.Stderr, "   Y (mm): %g <-> %g\n", miny, maxy)
	fmt.Fprintf(os.Stderr, "   Z (mm): %g <-> %g\n", minz, maxz)
	fmt.Fprintf(os.Stderr, "-------------------------\n")

}

//
// Application flow
//

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
	document, err := gcode.Parse(code)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parse error: %s\n", err)
		os.Exit(3)
	}

	// Run through the VM
	machine.Init()
	machine.MaxArcDeviation = *maxArcDeviation
	machine.MinArcLineLength = *minArcLineLength

	if err := machine.Process(document); err != nil {
		fmt.Fprintf(os.Stderr, "VM failed: %s\n", err)
		os.Exit(3)
	}

	// Optimize as requested
	if *opt {
		if *optDrillSpeed {
			optimize.OptDrillSpeed(&machine)
		}

		if *optRouteGrouping {
			if err := optimize.OptRouteGrouping(&machine, *rtolerance); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Could not execute route grouping: %s\n", err)
			}
		}

		if *optBogusMove {
			optimize.OptBogusMoves(&machine)
		}

		if *optVector {
			optimize.OptVector(&machine, *vtolerance)
		}

		if *optLiftSpeed {
			optimize.OptLiftSpeed(&machine)
		}
	}

	// Apply requested modifications
	if *flipXY {
		machine.FlipXY()
	}

	if *safetyHeight > 0 {
		if err := machine.SetSafetyHeight(*safetyHeight); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not set safety height%s\n", err)
		}
	}

	if *feedLimit > 0 {
		machine.LimitFeedrate(*feedLimit)
	}

	if *multiplyFeed != 0 {
		machine.FeedrateMultiplier(*multiplyFeed)
	}

	if *multiplyMove != 0 {
		machine.MoveMultiplier(*multiplyMove)
	}

	if *enforceReturn {
		machine.Return(true, true)
	}

	if *spindleCW > 0 {
		machine.EnforceSpindle(true, true, *spindleCW)
	} else if *spindleCCW > 0 {
		machine.EnforceSpindle(true, false, *spindleCCW)
	}

	if *stats {
		printStats(&machine)
	}

	// Handle VM output
	if *debugDump {
		machine.Dump()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Could not export vm state: %s\n", err)
		os.Exit(3)
	}

	if *dumpStdout {
		g := export.StringCodeGenerator{Precision: *precision}
		g.Init()
		export.HandleAllPositions(&machine, &g)
		fmt.Printf(g.Retrieve())
	}

	if *outputFile != "" {
		g := export.StringCodeGenerator{Precision: *precision}
		g.Init()
		export.HandleAllPositions(&machine, &g)

		if err := ioutil.WriteFile(*outputFile, []byte(g.Retrieve()), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Could not write to file: %s\n", err)
			os.Exit(2)
		}
	}

	if *device != "" {
		mt := &ManualGenerator{}
		wt := &WaitGenerator{}
		s := &streaming.GrblStreamer{}
		s.Precision = *precision

		generators = append(generators, mt)
		generators = append(generators, wt)
		generators = append(generators, s)

		s.Init()
		mt.Init()

		if err := s.Check(&machine); err != nil {
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

		pBar := pb.New(len(machine.Positions))
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

		for idx, _ := range machine.Positions {
			if err := export.HandlePositionAtIndex(&machine, idx, generators...); err != nil {
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
