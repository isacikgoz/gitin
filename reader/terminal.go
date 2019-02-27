package reader

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
