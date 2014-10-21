package gcode

import "strconv"
import "strings"
import "errors"

//
// The Node types
//

type Node interface {
	GetType() string
	Export(precision int) string
}

type Word struct {
	Address rune
	Command float64
}

type Comment struct {
	Content string
	EOL     bool
}

type Filemarker struct{}

//
// Methods
//

func (w *Word) GetType() string {
	return "word"
}

func (w *Word) Export(precision int) string {
	x := strconv.FormatFloat(w.Command, 'f', precision, 64)

	// Hacky way to remove silly zeroes
	if strings.IndexRune(x, '.') != -1 {
		for x[len(x)-1] == '0' {
			x = x[:len(x)-1]
		}
		if x[len(x)-1] == '.' {
			x = x[:len(x)-1]
		}
	}

	return string(w.Address) + x
}

func (c *Comment) GetType() string {
	return "comment"
}

func (c *Comment) Export(precision int) string {
	if c.EOL {
		return ";" + c.Content
	} else {
		return "(" + c.Content + ")"
	}
}

func (f *Filemarker) GetType() string {
	return "filemarker"
}

func (f *Filemarker) Export(precision int) string {
	return "%"
}

//
// Block type
//

type Block struct {
	Nodes       []Node
	BlockDelete bool
}

func (s *Block) AppendNode(n Node) {
	s.Nodes = append(s.Nodes, n)
}

func (s *Block) Export(precision int) string {
	var k string
	if s.BlockDelete {
		return ""
	}
	for _, c := range s.Nodes {
		k += c.Export(precision)
	}
	return k
}

func (s *Block) Length() int {
	return len(s.Nodes)
}

//
// Document type
//

type Document struct {
	Blocks []Block
}

func (doc *Document) AppendBlock(b Block) {
	doc.Blocks = append(doc.Blocks, b)
}

func (doc *Document) Export(precision int) string {
	l := make([]string, len(doc.Blocks))
	for idx, b := range doc.Blocks {
		l[idx] = b.Export(precision)
	}
	return strings.Join(l, "\n")
}

func (doc *Document) ExportMaxLength(precision, maxLength int) (string, error) {
	l := make([]string, len(doc.Blocks))
	origPrecision := precision
	for idx, b := range doc.Blocks {
		for {
			l[idx] = b.Export(precision)
			if precision == -1 {
				precision = maxLength
			} else if precision == 0 {
				return "", errors.New("Unable to reach maximum length")
			} else if len(l[idx]) > maxLength {
				precision--
			} else {
				precision = origPrecision
				break
			}
		}
	}
	return strings.Join(l, "\n"), nil
}

func (doc *Document) ToString() string {
	return doc.Export(-1)
}

func (doc *Document) Length() int {
	return len(doc.Blocks)
}

func Parse(input string) *Document {

	var (
		document    Document
		curBlock    Block = Block{}
		state       int   = 0
		lastNewline int   = 0
		buffer      string
		address     rune
	)

	parseNormal := func(c rune, idx int) {
		switch c {
		case '/':
			if idx-lastNewline == 0 {
				curBlock.BlockDelete = true
				lastNewline--
			} else {
				// TODO Error out!
			}
		case '%':
			fm := Filemarker{}
			curBlock.AppendNode(&fm)
		case '(':
			state = 1
		case ';':
			state = 2
		case '\n':
			document.AppendBlock(curBlock)
			curBlock = Block{}
			lastNewline = idx + 1
		case ' ':
			// Ignore
			return
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
			cm := Comment{buffer, false}
			curBlock.AppendNode(&cm)
			buffer = ""
		case '\n':
			// TODO Error out!
			state = 0
			cm := Comment{buffer, true}
			curBlock.AppendNode(&cm)
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
			cm := Comment{buffer, true}
			curBlock.AppendNode(&cm)
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
			w := Word{address, f}
			curBlock.AppendNode(&w)
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
