// Package e2e runs the gitin binary in a pseudo terminal like a user does:
// it types keys and checks what is drawn on the screen and what happened
// to the repository.
package e2e

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/isacikgoz/gitin/internal/gittest"
)

// keys gitin understands
const (
	enter     = "\r"
	down      = "\x1b[B"
	up        = "\x1b[A"
	ctrlC     = "\x03"
	ctrlU     = "\x15"
	backspace = "\x7f"
)

const timeout = 15 * time.Second

var (
	binary string
	// coverDir collects the coverage data of the gitin processes, if set
	coverDir = os.Getenv("GITIN_E2E_COVERDIR")
)

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	dir, err := os.MkdirTemp("", "gitin-e2e")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	binary = filepath.Join(dir, "gitin")

	if prebuilt := os.Getenv("GITIN_E2E_BINARY"); prebuilt != "" {
		// e.g. to check a release build or an older version
		binary = prebuilt
	} else {
		// build before git is isolated, the go command needs the real HOME
		args := []string{"build", "-o", binary}
		if coverDir != "" {
			args = append(args, "-cover", "-covermode=atomic", "-coverpkg=github.com/isacikgoz/gitin/...")
		}
		build := exec.Command("go", append(args, "github.com/isacikgoz/gitin/cmd/gitin")...)
		if out, err := build.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "building gitin: %v\n%s", err, out)
			return 1
		}
	}
	if err := gittest.Isolate(); err != nil {
		panic(err)
	}
	return m.Run()
}

// session is a gitin process running in a pseudo terminal
type session struct {
	t    *testing.T
	pty  *os.File
	cmd  *exec.Cmd
	mx   sync.Mutex
	out  bytes.Buffer
	read chan struct{} // closed when the terminal has no more output
	done chan struct{} // closed when the process exited
	err  error
}

// start runs gitin with args in dir, env is added to the environment
func start(t *testing.T, dir string, args []string, env ...string) *session {
	t.Helper()
	cmd := gitin(dir, args...)
	cmd.Env = append(cmd.Env, append([]string{"TERM=xterm-256color", "GIT_PAGER=cat", "PAGER=cat"}, env...)...)
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 40, Cols: 120})
	if err != nil {
		t.Fatal(err)
	}
	s := &session{t: t, pty: f, cmd: cmd, read: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(s.read)
		buf := make([]byte, 32*1024)
		for {
			n, err := f.Read(buf)
			s.mx.Lock()
			s.out.Write(buf[:n])
			s.mx.Unlock()
			if err != nil {
				return
			}
		}
	}()
	go func() {
		s.err = cmd.Wait()
		close(s.done)
	}()
	t.Cleanup(func() {
		select {
		case <-s.done:
		default:
			// quit like a user does, killed processes don't write coverage data
			_, _ = f.Write([]byte(ctrlC))
			select {
			case <-s.done:
			case <-time.After(2 * time.Second):
				_ = cmd.Process.Kill()
				<-s.done
			}
		}
		_ = f.Close()
		if t.Failed() {
			t.Logf("terminal output:\n%s", s.text())
		}
	})
	return s
}

func (s *session) send(keys ...string) {
	s.t.Helper()
	for _, k := range keys {
		if _, err := s.pty.Write([]byte(k)); err != nil {
			s.t.Fatalf("typing %q: %v", k, err)
		}
		// let gitin handle the key before the next one, like typing does
		time.Sleep(20 * time.Millisecond)
	}
}

var escapeSequence = regexp.MustCompile(`\x1b(\[[0-9;?]*[ -/]*[@-~]|[()][0-9A-Za-z]|[=>78])|\r`)

func (s *session) raw() string {
	s.mx.Lock()
	defer s.mx.Unlock()
	return s.out.String()
}

// text returns the output without escape sequences. gitin clears every line
// before drawing it, that starts a new line in the text.
func (s *session) text() string {
	return escapeSequence.ReplaceAllString(strings.ReplaceAll(s.raw(), "\x1b[2K", "\n"), "")
}

// mark returns the current end of the output, see waitAfter
func (s *session) mark() int {
	return len(s.text())
}

// frame returns the last screen gitin drew, it starts with the label
func (s *session) frame(label string, from int) string {
	out := s.text()
	if from > len(out) {
		from = len(out)
	}
	out = out[from:]
	i := strings.LastIndex(out, label)
	if i < 0 {
		return ""
	}
	return out[i:]
}

// waitFrame waits until the last screen gitin drew after mark contains all
// texts. Keys sent together draw a screen each, wait for a text only the
// last screen has, or use waitUntil.
func (s *session) waitFrame(label string, mark int, texts ...string) string {
	s.t.Helper()
	return s.waitUntil(label, mark, fmt.Sprintf("%q", texts), func(frame string) bool {
		return containsAll(frame, texts)
	})
}

// waitUntil waits until the last screen gitin drew after mark satisfies ok
func (s *session) waitUntil(label string, mark int, what string, ok func(frame string) bool) string {
	s.t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		frame := s.frame(label, mark)
		if frame != "" && ok(frame) {
			return frame
		}
		select {
		case <-s.done:
			<-s.read
			if frame = s.frame(label, mark); frame != "" && ok(frame) {
				return frame
			}
			s.t.Fatalf("gitin exited (%v) before showing %s", s.err, what)
		default:
		}
		if time.Now().After(deadline) {
			s.t.Fatalf("screen does not show %s, last screen:\n%s", what, frame)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// waitSelected waits until the item under the cursor ends with text
func (s *session) waitSelected(label string, mark int, text string) string {
	s.t.Helper()
	return s.waitUntil(label, mark, "the cursor on "+text, func(frame string) bool {
		return strings.HasSuffix(selectedLine(frame), text)
	})
}

// waitText waits until the output after mark contains all texts
func (s *session) waitText(mark int, texts ...string) {
	s.t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		out := s.text()
		if mark < len(out) && containsAll(out[mark:], texts) {
			return
		}
		if time.Now().After(deadline) {
			s.t.Fatalf("output does not show %q", texts)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// selectedLine returns the line of the item under the cursor in a frame
func selectedLine(frame string) string {
	for _, line := range strings.Split(frame, "\n") {
		if strings.HasPrefix(line, "> ") {
			return line
		}
	}
	return ""
}

func containsAll(s string, texts []string) bool {
	for _, text := range texts {
		if !strings.Contains(s, text) {
			return false
		}
	}
	return true
}

// wait waits for gitin to exit and returns its exit code
func (s *session) wait() int {
	s.t.Helper()
	select {
	case <-s.done:
	case <-time.After(timeout):
		s.t.Fatal("gitin did not exit")
	}
	<-s.read
	var exitErr *exec.ExitError
	if errors.As(s.err, &exitErr) {
		return exitErr.ExitCode()
	}
	if s.err != nil {
		s.t.Fatal(s.err)
	}
	return 0
}

// running reports whether gitin is still running after a short while
func (s *session) running() bool {
	select {
	case <-s.done:
		return false
	case <-time.After(300 * time.Millisecond):
		return true
	}
}

// gitin returns a command running gitin in dir
func gitin(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	if coverDir != "" {
		cmd.Env = append(cmd.Env, "GOCOVERDIR="+coverDir)
	}
	return cmd
}

// runGitin runs gitin without a terminal and returns its combined output
func runGitin(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()
	out, err := gitin(dir, args...).CombinedOutput()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), exitErr.ExitCode()
	}
	if err != nil && !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
	return string(out), 0
}
