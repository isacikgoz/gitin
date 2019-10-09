package term

import (
	"bufio"
	"bytes"
	"io"
	"syscall"

	"github.com/fatih/color"
)

var (
	state   terminalState
	reader  Reader
	writer  Writer
	colored = true
)

type terminalState struct {
	term   syscall.Termios
	reader *bufio.Reader
	buf    *bytes.Buffer
}

// Writer provides a minimal interface for Stdin.
type Writer interface {
	io.Writer
	Fd() uintptr
}

// Reader provides a minimal interface for Stdout.
type Reader interface {
	io.Reader
	Fd() uintptr
}

// Cell is a single character that will be drawn to the terminal
type Cell struct {
	Ch   rune
	Attr []color.Attribute
}

func newTerminalState(input Reader) terminalState {
	buf := new(bytes.Buffer)
	return terminalState{
		reader: bufio.NewReader(&BufferedReader{
			In:     input,
			Buffer: buf,
		}),
		buf: buf,
	}
}

// Cprint returns the text as colored cell slice
func Cprint(text string, attrs ...color.Attribute) []Cell {
	cells := make([]Cell, 0)
	for _, ch := range text {
		cells = append(cells, Cell{
			Ch:   ch,
			Attr: attrs,
		})
	}
	return cells
}

// DisableColor makes cell attributes meaningless
func DisableColor() {
	colored = false
}
