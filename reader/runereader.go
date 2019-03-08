// +build !windows

package reader

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"github.com/isacikgoz/sig/keys"
)

var (
	InterruptErr = errors.New("interrupt")
)

type RuneReader struct {
	Terminal Terminal
	state    runeReaderState
}

type runeReaderState struct {
	term   syscall.Termios
	reader *bufio.Reader
	buf    *bytes.Buffer
}

func NewRuneReader(terminal Terminal) *RuneReader {
	return &RuneReader{
		Terminal: terminal,
		state:    newRuneReaderState(terminal.In),
	}
}

func newRuneReaderState(input Reader) runeReaderState {
	buf := new(bytes.Buffer)
	return runeReaderState{
		reader: bufio.NewReader(&BufferedReader{
			In:     input,
			Buffer: buf,
		}),
		buf: buf,
	}
}

// For reading runes we just want to disable echo.
func (rr *RuneReader) SetTermMode() error {
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(rr.Terminal.In.Fd()), ioctlReadTermios, uintptr(unsafe.Pointer(&rr.state.term)), 0, 0, 0); err != 0 {
		return err
	}

	newState := rr.state.term
	newState.Lflag &^= syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.ISIG

	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(rr.Terminal.In.Fd()), ioctlWriteTermios, uintptr(unsafe.Pointer(&newState)), 0, 0, 0); err != 0 {
		return err
	}

	return nil
}

func (rr *RuneReader) RestoreTermMode() error {
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(rr.Terminal.In.Fd()), ioctlWriteTermios, uintptr(unsafe.Pointer(&rr.state.term)), 0, 0, 0); err != 0 {
		return err
	}
	return nil
}

func (rr *RuneReader) ReadRune() (rune, int, error) {
	r, size, err := rr.state.reader.ReadRune()
	if err != nil {
		return r, size, err
	}

	// parse ^[ sequences to look for arrow keys
	if r == '\033' {
		if rr.state.reader.Buffered() == 0 {
			// no more characters so must be `Esc` key
			return keys.Escape, 1, nil
		}
		r, size, err = rr.state.reader.ReadRune()
		if err != nil {
			return r, size, err
		}
		if r != '[' {
			return r, size, fmt.Errorf("Unexpected Escape Sequence: %q", []rune{'\033', r})
		}
		r, size, err = rr.state.reader.ReadRune()
		if err != nil {
			return r, size, err
		}
		switch r {
		case 'D':
			return keys.ArrowLeft, 1, nil
		case 'C':
			return keys.ArrowRight, 1, nil
		case 'A':
			return keys.ArrowUp, 1, nil
		case 'B':
			return keys.ArrowDown, 1, nil
		case 'H': // Home button
			return keys.Home, 1, nil
		case 'F': // End button
			return keys.End, 1, nil
		case '3': // Delete Button
			// discard the following '~' key from buffer
			rr.state.reader.Discard(1)
			return keys.Delete2, 1, nil
		default:
			// discard the following '~' key from buffer
			rr.state.reader.Discard(1)
			return keys.IgnoreKey, 1, nil
		}
	}
	return r, size, err
}
