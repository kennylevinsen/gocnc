package gcode

import "errors"
import "fmt"

type sliceOfWords []*Word

var (
	groups = map[string]sliceOfWords{
		"nonModalGroup": {&Word{'G', 4},
			&Word{'G', 10},
			&Word{'G', 28},
			&Word{'G', 30},
			&Word{'G', 53},
			&Word{'G', 92},
			&Word{'G', 92.1},
			&Word{'G', 92.2},
			&Word{'G', 92.3},
		},
		"motionGroup": {&Word{'G', 0},
			&Word{'G', 1},
			&Word{'G', 2},
			&Word{'G', 3},
			&Word{'G', 33},
			&Word{'G', 38.2},
			&Word{'G', 38.3},
			&Word{'G', 38.4},
			&Word{'G', 38.5},
			&Word{'G', 73},
			&Word{'G', 76},
			&Word{'G', 80},
			&Word{'G', 81},
			&Word{'G', 82},
			&Word{'G', 83},
			&Word{'G', 84},
			&Word{'G', 85},
			&Word{'G', 86},
			&Word{'G', 87},
			&Word{'G', 88},
			&Word{'G', 89},
		},
		"planeSelectionGroup": {&Word{'G', 17},
			&Word{'G', 18},
			&Word{'G', 19},
			&Word{'G', 17.1},
			&Word{'G', 18.1},
			&Word{'G', 19.1},
		},
		"distanceModeGroup": {&Word{'G', 90},
			&Word{'G', 91},
		},
		"arcDistanceModeGroup": {&Word{'G', 90.1},
			&Word{'G', 91.1},
		},
		"feedRateModeGroup": {&Word{'G', 93},
			&Word{'G', 94},
			&Word{'G', 95},
		},
		"unitsGroup": {&Word{'G', 20},
			&Word{'G', 21},
		},
		"cutterCompensationModeGroup": {&Word{'G', 40},
			&Word{'G', 41},
			&Word{'G', 41.1},
			&Word{'G', 42},
			&Word{'G', 42.1},
		},
		"toolLengthGroup": {&Word{'G', 43},
			&Word{'G', 43.1},
			&Word{'G', 49},
		},
		"cannedCyclesModeGroup": {&Word{'G', 98},
			&Word{'G', 99},
		},
		"coordinateSystemGroup": {&Word{'G', 54},
			&Word{'G', 55},
			&Word{'G', 56},
			&Word{'G', 57},
			&Word{'G', 58},
			&Word{'G', 59},
			&Word{'G', 59.1},
			&Word{'G', 59.2},
			&Word{'G', 59.3},
		},
		"controlModeGroup": {&Word{'G', 61},
			&Word{'G', 61.1},
			&Word{'G', 64},
		},
		"spindleModeGroup": {&Word{'G', 96},
			&Word{'G', 97},
		},
		"latheDiameterModeGroup": {&Word{'G', 7},
			&Word{'G', 8},
		},
		"stoppingGroup": {&Word{'M', 0},
			&Word{'M', 1},
			&Word{'M', 2},
			&Word{'M', 30},
			&Word{'M', 60},
		},
		"toolChangeGroup": {&Word{'M', 6},
			&Word{'M', 61},
		},
		"spindleGroup": {&Word{'M', 3},
			&Word{'M', 4},
			&Word{'M', 5},
		},
		"coolantGroup": {&Word{'M', 7},
			&Word{'M', 8},
			&Word{'M', 9},
		},
		"overrideGroup": {&Word{'M', 48},
			&Word{'M', 49},
			&Word{'M', 50},
			&Word{'M', 51},
			&Word{'M', 52},
			&Word{'M', 53},
		},
	}
)

func (n sliceOfWords) isInGroup(w *Word) bool {
	for _, word := range n {
		if *word == *w {
			return true
		}
	}
	return false
}

func (b *Block) GetModalGroup(t string) (*Word, error) {
	var word *Word
	group := groups[t]
	for _, n := range b.Nodes {
		if w, ok := n.(*Word); ok {
			if group.isInGroup(w) {
				if word != nil {
					return &Word{}, errors.New(fmt.Sprintf("Multiple gcodes from same modal group (%s)", t))
				}
				word = w
			}
		}
	}
	return word, nil
}
