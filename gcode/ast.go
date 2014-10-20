package gcode

import "strconv"

type GCode struct {
	address  rune
	command  float64
	content  string
	codetype string
}

func (g *GCode) Type() string {
	return g.codetype
}

func (g *GCode) Address() rune {
	return g.address
}

func (g *GCode) Command() float64 {
	return g.command
}

func (g *GCode) Content() string {
	return g.content
}

func (c *GCode) ToString() string {
	switch c.codetype {
	case "word":
		// TODO Strip unecessary decimal digits
		return string(c.address) + strconv.FormatFloat(c.command, 'f', -1, 64)
	case "filemarker":
		return "%"
	case "comment":
		return "(" + c.content + ")"
	}
	return ""
}

type GBlock struct {
	Codes       []GCode
	blockDelete bool
}

func (s *GBlock) AppendCode(c GCode) {
	s.Codes = append(s.Codes, c)
}

func (s *GBlock) ToString() string {
	var k string
	for _, c := range s.Codes {
		k += c.ToString()
	}
	return k
}

func (s *GBlock) Length() int {
	return len(s.Codes)
}

type GDocument struct {
	Blocks []GBlock
}

func (gdoc *GDocument) AppendBlock(b GBlock) {
	gdoc.Blocks = append(gdoc.Blocks, b)
}

func (gdoc *GDocument) ToString() string {
	var s string
	for _, b := range gdoc.Blocks {
		s += b.ToString() + "\n"
	}
	return s
}

func (gdoc *GDocument) Length() int {
	return len(gdoc.Blocks)
}

func Parse(input string) *GDocument {

	var (
		document    GDocument
		curBlock    GBlock = GBlock{}
		state       int    = 0
		lastNewline int    = 0
		buffer      string
		address     rune
	)

	parseNormal := func(c rune, idx int) {
		switch c {
		case '/':
			if idx-lastNewline == 0 {
				curBlock.blockDelete = true
				lastNewline--
			} else {
				// TODO Error out!
			}
		case '%':
			curBlock.AppendCode(GCode{0, 0, "", "filemarker"})
		case '(':
			state = 1
		case ';':
			state = 2
		case '\n':
			document.AppendBlock(curBlock)
			curBlock = GBlock{}
			lastNewline = idx
		default:
			if c >= 97 && c <= 122 {
				state = 3
				address = c - 32 // Make uppercase
			} else if (c >= 65 && c <= 90) || c == 64 || c == 94 {
				state = 3
				address = c
			} else {
				// TODO Error out!
			}
		}
	}

	parseComment := func(c rune, idx int) {
		switch c {
		case ')':
			state = 0
			curBlock.AppendCode(GCode{0, 0, buffer, "comment"})
			buffer = ""
		case '\n':
			// TODO Error out!
			state = 0
			curBlock.AppendCode(GCode{0, 0, buffer, "comment"})
			buffer = ""
			parseNormal(c, idx)
		default:
			buffer += string(c)
		}
	}

	parseEOLComment := func(c rune, idx int) {
		switch c {
		case '\n':
			state = 0
			curBlock.AppendCode(GCode{0, 0, buffer, "comment"})
			buffer = ""
			parseNormal(c, idx)
		default:
			buffer += string(c)
		}
	}

	parseWord := func(c rune, idx int) {
		if (c >= 48 && c <= 57) || c == 46 {
			buffer += string(c)
		} else {
			state = 0
			f, _ := strconv.ParseFloat(string(buffer), 64)
			curBlock.AppendCode(GCode{address, f, "", "word"})
			buffer = ""
			parseNormal(c, idx)
		}
	}

	for idx, c := range input + "\n" {
		switch state {
		case 0:
			parseNormal(c, idx)
		case 1:
			parseComment(c, idx)
		case 2:
			parseEOLComment(c, idx)
		case 3:
			parseWord(c, idx)
		}
	}
	return &document
}
