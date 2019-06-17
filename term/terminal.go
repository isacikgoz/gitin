package term

import (
	"bufio"
	"bytes"
	"io"
	"syscall"
	"unsafe"

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

// Init initializes the term package
func Init(r Reader, w Writer) error {
	reader = r
	writer = w
	state = newTerminalState(reader)
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(reader.Fd()), ioctlReadTermios, uintptr(unsafe.Pointer(&state.term)), 0, 0, 0); err != 0 {
		return err
	}

	newState := state.term
	// syscall.ECHO | syscall.ECHONL | syscall.ICANON to disable echo
	// syscall.ISIG is to catch keys like ctr-c or ctrl-d
	newState.Lflag &^= syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.ISIG

	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(reader.Fd()), ioctlWriteTermios, uintptr(unsafe.Pointer(&newState)), 0, 0, 0); err != 0 {
		return err
	}
	_, err := writer.Write([]byte(hideCursor))
	return err
}

// Close restores the terminal state
func Close() error {
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(reader.Fd()), ioctlWriteTermios, uintptr(unsafe.Pointer(&state.term)), 0, 0, 0); err != 0 {
		return err
	}
	_, err := writer.Write([]byte(showCursor))
	return err
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
