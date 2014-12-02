package gcode

import "fmt"
import "errors"
import "strconv"

// Parses a string, and returns an AST.
func Parse(input string) (doc *Document, err error) {

	const (
		normal     = iota
		comment    = iota
		eolcomment = iota
		word       = iota
	)

	var (
		document    Document
		curBlock    Block = Block{}
		state       int   = normal
		lastNewline int   = 0
		buffer      string
		address     rune
	)

	input += "\n"

	defer func() {
		if r := recover(); r != nil {
			err = errors.New(fmt.Sprintf("%s", r))
		}
	}()

	parserPanic := func(idx int, err string) string {
		nl := 0
		for idy, s := range input {
			if idy == idx {
				break
			} else if s == '\n' {
				nl++
			}
		}
		panic(fmt.Sprintf("Line %d, pos %d: %s", nl, idx-lastNewline+1, err))
	}

	parseNormal := func(c rune, idx int) {
		switch c {
		case '/':
			if idx-lastNewline == 0 {
				curBlock.BlockDelete = true
				lastNewline--
			} else {
				parserPanic(idx, "Unexpected /")
			}
		case '%':
			fm := Filemarker{}
			curBlock.AppendNode(&fm)
		case '(':
			state = comment
		case ';':
			state = eolcomment
		case '\n':
			document.AppendBlock(curBlock)
			curBlock = Block{}
			lastNewline = idx + 1
		case ' ':
			// Ignore
			return
		default:
			if c >= 97 && c <= 122 {
				// Lower-case character
				state = word
				address = c - 32 // Make uppercase
			} else if (c >= 65 && c <= 90) || c == 64 || c == 94 {
				// Upper-case character, @ or ^
				state = word
				address = c
			} else {
				// No clue
				parserPanic(idx, fmt.Sprintf("Expected word address, found %c", c))
			}
		}
	}

	parseComment := func(c rune, idx int) {
		switch c {
		case ')':
			state = normal
			cm := Comment{buffer, false}
			curBlock.AppendNode(&cm)
			buffer = ""
		case '\n':
			parserPanic(idx, "Non-terminated comment")
		default:
			buffer += string(c)
		}
	}

	parseEOLComment := func(c rune, idx int) {
		switch c {
		case '\n':
			state = normal
			cm := Comment{buffer, true}
			curBlock.AppendNode(&cm)
			buffer = ""
			parseNormal(c, idx)
		default:
			buffer += string(c)
		}
	}

	parseWord := func(c rune, idx int) {
		if (c >= 48 && c <= 57) || c == 46 || c == 45 || c == 43 {
			// [0-9\.\-\+]
			buffer += string(c)
		} else {
			// End of command
			state = normal
			f, _ := strconv.ParseFloat(string(buffer), 64)
			w := Word{address, f}
			curBlock.AppendNode(&w)
			buffer = ""
			parseNormal(c, idx)
		}
	}

	for idx, c := range input {
		switch state {
		case normal:
			parseNormal(c, idx)
		case comment:
			parseComment(c, idx)
		case eolcomment:
			parseEOLComment(c, idx)
		case word:
			parseWord(c, idx)
		}
	}
	return &document, nil
}
