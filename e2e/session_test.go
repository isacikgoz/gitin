// Package e2e runs the gitin binary in a pseudo terminal like a user does:
// it types keys and checks the screen of a terminal emulator, characters
// and colors, and what happened to the repository. Screens worth reviewing
// are kept in testdata/screens, go test ./e2e -update rewrites them.
package e2e

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"
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
	t   *testing.T
	pty *os.File
	cmd *exec.Cmd

	mx         sync.Mutex
	term       *vt.Emulator
	hidden     bool      // the cursor is hidden
	received   int       // bytes of output so far
	lastOutput time.Time // when output arrived last
	sent       int       // received when keys were sent last

	read chan struct{} // closed when the terminal has no more output
	done chan struct{} // closed when the process exited
	err  error
}

// start runs gitin with args in dir, env is added to the environment
func start(t *testing.T, dir string, args []string, env ...string) *session {
	t.Helper()
	cmd := gitin(dir, args...)
	cmd.Env = append(cmd.Env, append([]string{"TERM=xterm-256color", "GIT_PAGER=cat", "PAGER=cat"}, env...)...)
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: rows, Cols: cols})
	if err != nil {
		t.Fatal(err)
	}
	s := &session{t: t, pty: f, cmd: cmd, term: vt.NewEmulator(cols, rows), read: make(chan struct{}), done: make(chan struct{})}
	// called while the output is written, with s.mx held
	s.term.SetCallbacks(vt.Callbacks{CursorVisibility: func(visible bool) { s.hidden = !visible }})
	// the terminal answers queries like the cursor position, the goroutine
	// ends with the tests: closing the emulator races with its reads
	go func() { _, _ = io.Copy(f, s.term) }()
	go func() {
		defer close(s.read)
		buf := make([]byte, 32*1024)
		for {
			n, err := f.Read(buf)
			s.mx.Lock()
			_, _ = s.term.Write(buf[:n])
			s.received += n
			s.lastOutput = time.Now()
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
			t.Logf("terminal output:\n%s", s.output())
		}
	})
	return s
}

func (s *session) send(keys ...string) {
	s.t.Helper()
	s.mx.Lock()
	s.sent = s.received
	s.mx.Unlock()
	for _, k := range keys {
		if _, err := s.pty.Write([]byte(k)); err != nil {
			s.t.Fatalf("typing %q: %v", k, err)
		}
		// let gitin handle the key before the next one, like typing does
		time.Sleep(20 * time.Millisecond)
	}
}

// drawn returns the screen and whether gitin drew it: it has drawn since
// the last keys were sent and stopped drawing
func (s *session) drawn() (string, bool) {
	s.mx.Lock()
	defer s.mx.Unlock()
	return strings.Join(s.screenRows(), "\n"), s.received > s.sent && time.Since(s.lastOutput) >= settle
}

// waitScreen waits until gitin drew a screen with all texts and returns it
func (s *session) waitScreen(texts ...string) string {
	s.t.Helper()
	return s.waitUntil(fmt.Sprintf("%q", texts), func(screen string) bool {
		return containsAll(screen, texts)
	})
}

// waitUntil waits until gitin drew a screen that satisfies ok and returns it
func (s *session) waitUntil(what string, ok func(screen string) bool) string {
	s.t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		screen, drawn := s.drawn()
		if drawn && ok(screen) {
			return screen
		}
		select {
		case <-s.done:
			<-s.read
			if screen = s.screen(); ok(screen) {
				return screen
			}
			s.t.Fatalf("gitin exited (%v) before showing %s, last screen:\n%s", s.err, what, screen)
		default:
		}
		if time.Now().After(deadline) {
			s.t.Fatalf("the screen does not show %s:\n%s", what, screen)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// waitSelected waits until the item under the cursor ends with text
func (s *session) waitSelected(text string) string {
	s.t.Helper()
	return s.waitUntil("the cursor on "+text, func(screen string) bool {
		return strings.HasSuffix(selectedLine(screen), text)
	})
}

// waitOutput waits until the output, see output, contains all texts
func (s *session) waitOutput(texts ...string) {
	s.t.Helper()
	deadline := time.Now().Add(timeout)
	for !containsAll(s.output(), texts) {
		if time.Now().After(deadline) {
			s.t.Fatalf("the output does not show %q:\n%s", texts, s.output())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// selectedLine returns the line of the item under the cursor, the last one
// starting with "> "
func selectedLine(screen string) string {
	lines := strings.Split(screen, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.HasPrefix(lines[i], "> ") {
			return lines[i]
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
