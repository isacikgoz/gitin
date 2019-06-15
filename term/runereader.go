// +build !windows
// This is a modified version of survey's runereader. The original version can
// be found at https://github.com/AlecAivazis/survey

package term

import (
	"fmt"
)

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

// ReadRune returns a single rune from the stdin
func (rr *RuneReader) ReadRune() (rune, int, error) {
	r, size, err := state.reader.ReadRune()
	if err != nil {
		return r, size, err
	}

	// parse ^[ sequences to look for arrow keys
	if r == '\033' {
		if state.reader.Buffered() == 0 {
			// no more characters so must be `Esc` key
			return rune(KeyESC), 1, nil
		}
		r, size, err = state.reader.ReadRune()
		if err != nil {
			return r, size, err
		}
		if r != '[' {
			return r, size, fmt.Errorf("Unexpected Escape Sequence: %q", []rune{'\033', r})
		}
		r, size, err = state.reader.ReadRune()
		if err != nil {
			return r, size, err
		}
		switch r {
		case 'D':
			return ArrowLeft, 1, nil
		case 'C':
			return ArrowRight, 1, nil
		case 'A':
			return ArrowUp, 1, nil
		case 'B':
			return ArrowDown, 1, nil
		case 'H': // Home button
			return rune(KeyCtrlA), 1, nil
		case 'F': // End button
			return rune(KeyCtrlQ), 1, nil
		case '3': // Delete Button
			// discard the following '~' key from buffer
			state.reader.Discard(1)
			return rune(KeyCtrlR), 1, nil
		default:
			// discard the following '~' key from buffer
			state.reader.Discard(1)
			return rune(KeyCtrlSpace), 1, nil
		}
	}
	return r, size, err
}
