package gcode

import "strconv"
import "strings"
import "errors"
import "fmt"

//
// The Node types
//

// An interface covering Word, Comment and Filemarker.
type Node interface {
	GetType() string
	Export(precision int) string
}

// A Gcode word (Such as "G0", or "X-103.4").
type Word struct {
	Address rune
	Command float64
}

// A comment (Such as "(Hello)", or ";Hello").
type Comment struct {
	Content string
	EOL     bool
}

// A file marker (Does not contain any other parameters).
type Filemarker struct{}

//
// Methods
//

func (w *Word) GetType() string {
	return "word"
}

// Exports the word as-is, using the given floating point precision.
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

// Exports the comment as is.
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

// Exports the filemarker (just returns "%", really).
func (f *Filemarker) Export(precision int) string {
	return "%"
}

//
// Block type
//

// A block, which is a slice of Nodes.
type Block struct {
	Nodes       []Node
	BlockDelete bool
}

// Append a node to the block.
func (s *Block) AppendNode(n Node) {
	s.Nodes = append(s.Nodes, n)
}

// Append multiple nodes ot the block.
func (s *Block) AppendNodes(n ...Node) {
	for _, m := range n {
		s.AppendNode(m)
	}
}

// Exports the block, using the provided floating point precision. Respects block-delete.
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

// Other utility functions

// Finds a word with the specified address.
func (s *Block) GetWord(address rune) (res float64, err error) {
	found := false
	for _, m := range s.Nodes {
		if word, ok := m.(*Word); ok {
			if word.Address == address {
				if found {
					return res, errors.New(fmt.Sprintf("Multiple instances of address '%c' in block", address))
				}
				found = true
				res = word.Command
			}
		}
	}
	if !found {
		return res, errors.New(fmt.Sprintf("'%c' not found in block", address))
	}
	return res, nil
}

// Same as GetWord, but has a default value.
func (s *Block) GetWordDefault(address rune, def float64) (res float64) {
	res, err := s.GetWord(address)
	if err != nil {
		return def
	}
	return res
}

// Retrieves all words with the specified address.
func (s *Block) GetAllWords(address rune) (res []float64) {
	for _, m := range s.Nodes {
		if word, ok := m.(*Word); ok {
			if word.Address == address {
				res = append(res, word.Command)
			}
		}
	}
	return res
}

// Tests if one of the given addresses exist.
func (s *Block) IncludesOneOf(addresses ...rune) (res bool) {
	for _, m := range addresses {
		_, err := s.GetWord(m)
		if err == nil {
			return true
		}
	}
	return false
}

// Test if the specific word exists.
func (s *Block) HasWord(address rune, command float64) (res bool) {
	for _, m := range s.Nodes {
		if word, ok := m.(*Word); ok {
			if word.Address == address && word.Command == command {
				return true
			}
		}
	}
	return false
}

//
// Document type
//

// A document, which is a slice of Blocks.
type Document struct {
	Blocks []Block
}

// Append a block to the document.
func (doc *Document) AppendBlock(b Block) {
	doc.Blocks = append(doc.Blocks, b)
}

// Exports the document, using the provided floating point precision. Respects block-delete.
func (doc *Document) Export(precision int) string {
	l := make([]string, len(doc.Blocks))
	for idx, b := range doc.Blocks {
		l[idx] = b.Export(precision)
	}
	return strings.Join(l, "\n")
}

// Like Export, but uses as many digits as necessary for floating point.
func (doc *Document) ToString() string {
	return doc.Export(-1)
}

func (doc *Document) Length() int {
	return len(doc.Blocks)
}
