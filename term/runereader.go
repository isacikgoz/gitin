//go:build !windows

// This is a modified version of survey's runereader. The original version can
// be found at https://github.com/AlecAivazis/survey

package term

import (
	"bufio"
	"errors"
	"time"

	"golang.org/x/sys/unix"
)

// ErrNoKey is returned by ReadRuneTimeout when no key was pressed in time
var ErrNoKey = errors.New("no key pressed")

// RuneReader reads from an io.Reader interface
type RuneReader struct {
	in Reader
}

// NewRuneReader creates a new instance of RuneReader
func NewRuneReader(reader Reader) *RuneReader {
	return &RuneReader{
		in: reader,
	}
}

// ReadRune returns a single key from the stdin. Escape sequences of special
// keys are translated to their aliases, unknown ones to KeyCtrlSpace which
// has no binding.
func (rr *RuneReader) ReadRune() (rune, int, error) {
	return readKey(state.reader)
}

// ReadRuneTimeout is ReadRune, but it returns ErrNoKey if no key is pressed
// within timeout. Unlike a blocked ReadRune, it lets the caller stop reading.
func (rr *RuneReader) ReadRuneTimeout(timeout time.Duration) (rune, int, error) {
	if state.reader.Buffered() == 0 {
		ready, err := waitForInput(int(reader.Fd()), timeout)
		if err != nil {
			return 0, 0, err
		}
		if !ready {
			return 0, 0, ErrNoKey
		}
	}
	return readKey(state.reader)
}

// waitForInput waits until fd can be read from or the timeout passes. It
// uses select because poll doesn't support terminals on macOS.
func waitForInput(fd int, timeout time.Duration) (bool, error) {
	deadline := time.Now().Add(timeout)
	for {
		var fds unix.FdSet
		fds.Set(fd)
		tv := unix.NsecToTimeval(max(time.Until(deadline), 0).Nanoseconds())
		n, err := unix.Select(fd+1, &fds, nil, nil, &tv)
		if errors.Is(err, unix.EINTR) {
			continue // e.g. the terminal was resized
		}
		return n > 0, err
	}
}

// maxSequenceLength limits how much input an unexpected escape sequence consumes
const maxSequenceLength = 16

func readKey(in *bufio.Reader) (rune, int, error) {
	r, size, err := in.ReadRune()
	if err != nil || r != '\033' {
		return r, size, err
	}
	if in.Buffered() == 0 {
		// no more characters so must be `Esc` key
		return rune(KeyESC), 1, nil
	}
	r, _, err = in.ReadRune()
	if err != nil {
		return r, 1, err
	}
	switch r {
	case '[': // CSI, e.g. "\033[A" or "\033[1;5C" with modifiers
		var params []rune
		for i := 0; i < maxSequenceLength; i++ {
			r, _, err = in.ReadRune()
			if err != nil {
				return r, 1, err
			}
			if r >= 0x40 && r <= 0x7e { // final byte
				return csiKey(r, string(params)), 1, nil
			}
			params = append(params, r)
		}
	case 'O': // SS3, arrows in application cursor key mode, e.g. "\033OA"
		r, _, err = in.ReadRune()
		if err != nil {
			return r, 1, err
		}
		return csiKey(r, ""), 1, nil
	}
	// Alt+key or an unknown sequence
	return rune(KeyCtrlSpace), 1, nil
}

func csiKey(final rune, params string) rune {
	switch final {
	case 'D':
		return ArrowLeft
	case 'C':
		return ArrowRight
	case 'A':
		return ArrowUp
	case 'B':
		return ArrowDown
	case 'H': // Home button
		return rune(KeyCtrlA)
	case 'F': // End button
		return rune(KeyCtrlQ)
	case '~':
		switch params {
		case "1", "7": // Home button
			return rune(KeyCtrlA)
		case "4", "8": // End button
			return rune(KeyCtrlQ)
		case "3": // Delete button
			return rune(KeyCtrlR)
		}
	}
	return rune(KeyCtrlSpace)
}
