// Package term is influenced by https://github.com/AlecAivazis/survey and
// https://github.com/manifoldco/promptui it might contain some code snippets from those
// A little copying is better than a little dependency. - Go proverbs.
package term

import (
	"bytes"
	"fmt"
	"io"

	"github.com/fatih/color"
)

// BufferedWriter creates, clears and, moves up or down lines as needed to write
// the output to the terminal using ANSI escape codes.
type BufferedWriter struct {
	w        io.Writer
	buf      *bytes.Buffer
	lineWrap bool
	reset    bool
	flush    bool
	cursor   int
	height   int
}

// NewBufferedWriter creates and initializes a new BufferedWriter.
func NewBufferedWriter(w io.Writer) *BufferedWriter {
	return &BufferedWriter{buf: &bytes.Buffer{}, w: w}
}

// Reset truncates the underlining buffer and marks all its previous lines to be
// cleared during the next Write.
func (b *BufferedWriter) Reset() {
	b.buf.Reset()
	b.reset = true
}

// Write writes a single line to the underlining buffer.
func (b *BufferedWriter) Write(bites []byte) (int, error) {
	if bytes.ContainsAny(bites, "\r\n") {
		return 0, fmt.Errorf("%q should not contain either \\r or \\n", bites)
	}

	if !b.lineWrap {
		b.buf.Write([]byte(lwoff))
		defer b.buf.Write([]byte(lwon))
	}

	if b.reset {
		for i := 0; i < b.height; i++ {
			_, err := b.buf.Write(moveUp)
			if err != nil {
				return 0, err
			}
			_, err = b.buf.Write(clearLine)
			if err != nil {
				return 0, err
			}
		}
		b.cursor = 0
		b.height = 0
		b.reset = false
	}

	switch {
	case b.cursor == b.height:
		n, err := b.buf.Write(clearLine)
		if err != nil {
			return n, err
		}
		line := append(bites, []byte("\n")...)
		n, err = b.buf.Write(line)
		if err != nil {
			return n, err
		}
		b.height++
		b.cursor++
		return n, nil
	case b.cursor < b.height:
		n, err := b.buf.Write(clearLine)
		if err != nil {
			return n, err
		}
		n, err = b.buf.Write(bites)
		if err != nil {
			return n, err
		}
		n, err = b.buf.Write(moveDown)
		if err != nil {
			return n, err
		}
		b.cursor++
		return n, nil
	default:
		return 0, fmt.Errorf("Invalid write cursor position (%d) exceeded line height: %d", b.cursor, b.height)
	}
}

// WriteCells add colored text to the inner buffer
func (b *BufferedWriter) WriteCells(cs []Cell) (int, error) {
	bs := make([]byte, 0)
	if colored {
		for _, c := range cs {
			paint := color.New(c.Attr...)
			painted := paint.Sprintf(string(c.Ch))
			bs = append(bs, []byte(painted)...)
		}
	} else {
		for _, c := range cs {
			bs = append(bs, []byte(string(c.Ch))...)
		}
	}
	return b.Write(bs)
}

// Flush writes any buffered data to the underlying io.Writer, ensuring that any pending data is displayed.
func (b *BufferedWriter) Flush() error {
	for i := b.cursor; i < b.height; i++ {
		if i < b.height {
			_, err := b.buf.Write(clearLine)
			if err != nil {
				return err
			}
		}
		_, err := b.buf.Write(moveDown)
		if err != nil {
			return err
		}
	}

	_, err := b.buf.WriteTo(b.w)
	if err != nil {
		return err
	}
	b.buf.Reset()
	// reset cursor position
	b.buf.Write(clearLine)
	_, err = b.buf.WriteTo(b.w)
	if err != nil {
		return err
	}
	b.buf.Reset()

	for i := 0; i < b.height; i++ {
		_, err := b.buf.Write(moveUp)
		if err != nil {
			return err
		}
	}

	b.cursor = 0

	return nil
}

// ClearScreen solves problems (R) and use it after Reset()
func (b *BufferedWriter) ClearScreen() error {
	for i := 0; i < b.height; i++ {
		_, err := b.buf.Write(moveUp)
		if err != nil {
			return err
		}
		_, err = b.buf.Write(clearLine)
		if err != nil {
			return err
		}
	}
	b.cursor = 0
	b.height = 0
	b.reset = false

	_, err := b.buf.WriteTo(b.w)
	if err != nil {
		return err
	}
	b.buf.Reset()
	return nil
}

// ShowCursor writes to os.Stdout that to show cursor
func (b *BufferedWriter) ShowCursor() {
	b.w.Write([]byte(showCursor))
}

// HideCursor writes to os.Stdout that to hide cursor
func (b *BufferedWriter) HideCursor() {
	b.w.Write([]byte(hideCursor))
}
