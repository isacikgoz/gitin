package term

import (
	"io"

	"github.com/fatih/color"
)

// Terminal is the standard input/output the terminal reads/writes with.
type Terminal struct {
	In  Reader
	Out Writer
	Err io.Writer
}

// Cell is a single character that will be drawn to the terminal
type Cell struct {
	Ch   rune
	Attr []color.Attribute
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

func (t *Terminal) LineWrapOff() {
	t.Out.Write([]byte(lwoff))
}

func (t *Terminal) LineWrapOn() {
	t.Out.Write([]byte(lwon))
}

func (t *Terminal) ShowCursor() {
	t.Out.Write([]byte(showCursor))
}

func (t *Terminal) HideCursor() {
	t.Out.Write([]byte(hideCursor))
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
