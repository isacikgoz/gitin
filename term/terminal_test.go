package term

import (
	"bytes"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/creack/pty"
	"github.com/fatih/color"
)

// screen interprets the escape sequences BufferedWriter writes
type screen struct {
	lines []string
	row   int
}

func (s *screen) feed(out string) {
	grow := func() {
		for len(s.lines) <= s.row {
			s.lines = append(s.lines, "")
		}
	}
	grow()
	for len(out) > 0 {
		switch {
		case strings.HasPrefix(out, string(clearLine)):
			s.lines[s.row] = ""
			out = out[len(clearLine):]
		case strings.HasPrefix(out, string(moveUp)):
			s.row = max(s.row-1, 0) // like terminals, stop at the top
			out = out[len(moveUp):]
		case strings.HasPrefix(out, string(moveDown)):
			s.row++
			grow()
			out = out[len(moveDown):]
		case strings.HasPrefix(out, "\x1b["):
			end := strings.IndexAny(out[2:], "ABCDHJKfhlmsu")
			out = out[end+3:] // colors, line wrap
		case out[0] == '\n':
			s.row++
			grow()
			out = out[1:]
		default:
			s.lines[s.row] += out[:1]
			out = out[1:]
		}
	}
}

func (s *screen) text() string {
	return strings.TrimRight(strings.Join(s.lines, "\n"), "\n")
}

func TestBufferedWriterRedraws(t *testing.T) {
	var out bytes.Buffer
	var scr screen
	w := NewBufferedWriter(&out)
	frame := func(lines ...string) string {
		t.Helper()
		for _, line := range lines {
			if _, err := w.Write([]byte(line)); err != nil {
				t.Fatal(err)
			}
		}
		if err := w.Flush(); err != nil {
			t.Fatal(err)
		}
		scr.feed(out.String())
		out.Reset()
		return scr.text()
	}

	if got := frame("first", "second", "third"); got != "first\nsecond\nthird" {
		t.Fatalf("got %q", got)
	}
	// a shorter frame clears the lines of the previous one
	if got := frame("one"); got != "one" {
		t.Fatalf("got %q", got)
	}
	if got := frame("a", "b"); got != "a\nb" {
		t.Fatalf("got %q", got)
	}

	w.Reset()
	if got := frame("after reset"); got != "after reset" {
		t.Fatalf("got %q", got)
	}

	// what the prompt does when it exits
	w.Reset()
	if err := w.ClearScreen(); err != nil {
		t.Fatal(err)
	}
	scr.feed(out.String())
	if got := scr.text(); got != "" {
		t.Fatalf("got %q after clearing the screen", got)
	}
}

func TestBufferedWriterRejectsLineBreaks(t *testing.T) {
	w := NewBufferedWriter(&bytes.Buffer{})
	for _, line := range []string{"a\nb", "a\rb"} {
		if _, err := w.Write([]byte(line)); err == nil {
			t.Errorf("%q: no error", line)
		}
	}
}

func TestBufferedWriterDoesNotModifyInput(t *testing.T) {
	w := NewBufferedWriter(&bytes.Buffer{})
	backing := []byte("abcdef")
	if _, err := w.Write(backing[:3]); err != nil {
		t.Fatal(err)
	}
	if string(backing) != "abcdef" {
		t.Fatalf("Write changed the caller's buffer to %q", backing)
	}
}

func TestWriteCellsWithoutColors(t *testing.T) {
	defer func(noColor bool) { color.NoColor = noColor }(color.NoColor)
	color.NoColor = false
	DisableColor()
	defer func() { colored = true }()

	var out bytes.Buffer
	w := NewBufferedWriter(&out)
	if _, err := w.WriteCells(Cprint("plain", color.FgRed, color.Bold)); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "\x1b[31") || !strings.Contains(out.String(), "plain") {
		t.Fatalf("got %q", out.String())
	}
}

func TestCursorVisibility(t *testing.T) {
	var out bytes.Buffer
	w := NewBufferedWriter(&out)
	w.HideCursor()
	w.ShowCursor()
	if out.String() != hideCursor+showCursor {
		t.Fatalf("got %q", out.String())
	}
}

func TestCprint(t *testing.T) {
	cells := Cprint("añ", color.FgGreen)
	if len(cells) != 2 || cells[1].Ch != 'ñ' || cells[0].Attr[0] != color.FgGreen {
		t.Fatalf("got %+v", cells)
	}
	if len(Cprint("")) != 0 {
		t.Fatal("empty text has cells")
	}
}

// ptyOutput collects what is written to the terminal
type ptyOutput struct {
	mx  sync.Mutex
	buf bytes.Buffer
}

func (o *ptyOutput) wait(t *testing.T, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		o.mx.Lock()
		got := o.buf.String()
		o.mx.Unlock()
		if strings.Contains(got, want) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("terminal got %q, want %q", got, want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func lflag(t *testing.T, tty *os.File) uint64 {
	t.Helper()
	var termios syscall.Termios
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, tty.Fd(), ioctlReadTermios, uintptr(unsafe.Pointer(&termios)), 0, 0, 0); err != 0 {
		t.Fatal(err)
	}
	return uint64(termios.Lflag)
}

func TestInitAndClose(t *testing.T) {
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Skipf("no pseudo terminal: %v", err)
	}
	defer func() { _ = ptmx.Close() }()
	defer func() { _ = tty.Close() }()
	out := &ptyOutput{}
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := ptmx.Read(buf)
			out.mx.Lock()
			out.buf.Write(buf[:n])
			out.mx.Unlock()
			if err != nil {
				return
			}
		}
	}()

	raw := uint64(syscall.ECHO | syscall.ICANON | syscall.ISIG)
	if lflag(t, tty)&raw != raw {
		t.Fatal("terminal does not start in cooked mode")
	}
	if err := Init(tty, tty); err != nil {
		t.Fatal(err)
	}
	if lflag(t, tty)&raw != 0 {
		t.Fatal("Init did not turn off echo, line buffering and signals")
	}
	out.wait(t, hideCursor)

	// keys are read one by one, special keys are translated
	if _, err := ptmx.Write([]byte("q\x1b[A\x1bOB")); err != nil {
		t.Fatal(err)
	}
	rr := NewRuneReader(tty)
	for _, want := range []rune{'q', ArrowUp, ArrowDown} {
		r, _, err := rr.ReadRune()
		if err != nil || r != want {
			t.Fatalf("got %q, %v, want %q", r, err, want)
		}
	}

	if err := Close(); err != nil {
		t.Fatal(err)
	}
	if lflag(t, tty)&raw != raw {
		t.Fatal("Close did not restore the terminal")
	}
	out.wait(t, showCursor)
}

func TestInitWithoutTerminal(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "not-a-tty")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if err := Init(f, f); err == nil {
		t.Fatal("Init accepted a regular file")
	}
}
