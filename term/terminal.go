package term

import (
	"io"
)

// Terminal is the standard input/output the terminal reads/writes with.
type Terminal struct {
	In  Reader
	Out Writer
	Err io.Writer
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
