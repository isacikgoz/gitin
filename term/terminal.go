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
	input Reader
	// keys buffers the input, it is kept for the next prompt so keys typed
	// ahead are not lost
	keys   *bufio.Reader
	output Writer
	// saved are the terminal modes before each Init, the first is the
	// user's
	saved   []syscall.Termios
	colored = true
)

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

// Init puts the terminal in raw mode: keys are read one by one, without
// echo, and Ctrl-C is a key instead of a signal. Calls can be nested, every
// Init needs a Close.
func Init(r Reader, w Writer) error {
	if input != r || keys == nil {
		input, keys = r, bufio.NewReader(&BufferedReader{In: r, Buffer: new(bytes.Buffer)})
	}
	output = w
	current, err := getMode(r.Fd())
	if err != nil {
		return err
	}
	saved = append(saved, current)
	mode := raw(current)
	if err := setMode(r.Fd(), &mode); err != nil {
		saved = saved[:len(saved)-1]
		return err
	}
	_, err = w.Write([]byte(hideCursor))
	return err
}

// Close restores the terminal mode from before the matching Init
func Close() error {
	if len(saved) == 0 {
		return nil
	}
	mode := saved[len(saved)-1]
	saved = saved[:len(saved)-1]
	if err := setMode(input.Fd(), &mode); err != nil {
		return err
	}
	_, err := output.Write([]byte(showCursor))
	return err
}

// Suspend gives the terminal back in the user's mode, e.g. to run a command
// that prints or reads input. Resume takes it back.
func Suspend() error {
	if len(saved) == 0 {
		return nil
	}
	if err := setMode(input.Fd(), &saved[0]); err != nil {
		return err
	}
	_, err := output.Write([]byte(showCursor))
	return err
}

// Resume puts the terminal in raw mode again after Suspend
func Resume() error {
	if len(saved) == 0 {
		return nil
	}
	mode := raw(saved[0])
	if err := setMode(input.Fd(), &mode); err != nil {
		return err
	}
	_, err := output.Write([]byte(hideCursor))
	return err
}

func raw(mode syscall.Termios) syscall.Termios {
	// syscall.ECHO | syscall.ECHONL | syscall.ICANON to disable echo
	// syscall.ISIG is to catch keys like ctr-c or ctrl-d
	mode.Lflag &^= syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.ISIG
	return mode
}

func getMode(fd uintptr) (syscall.Termios, error) {
	var mode syscall.Termios
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, fd, ioctlReadTermios, uintptr(unsafe.Pointer(&mode)), 0, 0, 0); err != 0 {
		return mode, err
	}
	return mode, nil
}

func setMode(fd uintptr, mode *syscall.Termios) error {
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, fd, ioctlWriteTermios, uintptr(unsafe.Pointer(mode)), 0, 0, 0); err != 0 {
		return err
	}
	return nil
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
