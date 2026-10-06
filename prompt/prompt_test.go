package prompt

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fatih/color"
	"github.com/isacikgoz/gitin/term"
)

// syncBuffer is a bytes.Buffer that can be read while the prompt writes
type syncBuffer struct {
	mx  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mx.Lock()
	defer b.mx.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mx.Lock()
	defer b.mx.Unlock()
	return b.buf.String()
}

var escapeSequence = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)

type testPrompt struct {
	*Prompt
	out      *syncBuffer
	done     chan error
	selected []interface{}
	handled  []string
}

// startPrompt runs the main loop of a prompt over the given items, keys are
// fed with send instead of a terminal.
func startPrompt(t *testing.T, list List, opts *Options) *testPrompt {
	t.Helper()
	tp := &testPrompt{out: &syncBuffer{}, done: make(chan error, 1)}
	tp.Prompt = Create("Items", opts, list,
		WithSelectionHandler(func(item interface{}) error {
			tp.selected = append(tp.selected, item)
			return nil
		}),
		WithInformation(func(item interface{}) [][]term.Cell {
			return [][]term.Cell{term.Cprint(fmt.Sprintf("info of %s", item))}
		}),
	)
	tp.writer = term.NewBufferedWriter(tp.out)
	for _, kb := range []*KeyBinding{
		{Key: 'x', Display: "x", Desc: "mark", Handler: func(item interface{}) error {
			tp.handled = append(tp.handled, fmt.Sprint(item))
			return nil
		}},
		{Key: 'e', Display: "e", Desc: "fail", Handler: func(item interface{}) error {
			return errors.New("something failed\nhint: try again\n")
		}},
	} {
		if err := tp.AddKeyBinding(kb); err != nil {
			t.Fatal(err)
		}
	}
	tp.render()
	go func() { tp.done <- tp.mainloop() }()
	t.Cleanup(func() {
		tp.Stop()
		<-tp.done
	})
	return tp
}

func (tp *testPrompt) send(keys ...rune) {
	for _, k := range keys {
		tp.events <- keyEvent{ch: k}
	}
}

// frame returns the text of the last rendered screen
func (tp *testPrompt) frame(label string) string {
	out := escapeSequence.ReplaceAllString(tp.out.String(), "")
	if i := strings.LastIndex(out, label); i >= 0 {
		out = out[i:]
	}
	return out
}

// waitFrame waits until the last rendered screen contains all the texts
func (tp *testPrompt) waitFrame(t *testing.T, label string, texts ...string) string {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		frame := tp.frame(label)
		missing := ""
		for _, text := range texts {
			if !strings.Contains(frame, text) {
				missing = text
			}
		}
		if missing == "" {
			return frame
		}
		if time.Now().After(deadline) {
			t.Fatalf("screen does not show %q:\n%s", missing, frame)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// stop quits the prompt and returns what its main loop returned
func (tp *testPrompt) stop(t *testing.T) error {
	t.Helper()
	tp.send(rune(term.KeyCtrlC))
	select {
	case err := <-tp.done:
		tp.done <- err // for the cleanup
		return err
	case <-timeout():
		t.Fatal("prompt did not stop")
		return nil
	}
}

func syncList(t *testing.T, items []string, size int) *SyncList {
	t.Helper()
	l, err := NewList(items, size)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestPromptNavigationAndSelection(t *testing.T) {
	tp := startPrompt(t, syncList(t, numbered(10), 3), &Options{LineSize: 3, VimKeys: true})
	tp.waitFrame(t, "Items", "> item-0", "info of item-0")

	tp.send('j', 'j', term.ArrowDown)
	tp.waitFrame(t, "Items", "> item-3", "info of item-3")
	tp.send('k', term.ArrowUp)
	tp.waitFrame(t, "Items", "> item-1", "item-3")
	tp.send('h') // page down moves the cursor to the first item of the next page
	tp.waitFrame(t, "Items", "> item-4", "item-6")
	tp.send(term.ArrowRight) // page up
	tp.waitFrame(t, "Items", "> item-1", "item-3")
	tp.send(term.ArrowLeft) // page down
	tp.waitFrame(t, "Items", "> item-4", "item-6")
	tp.send('l') // page up
	tp.waitFrame(t, "Items", "> item-1", "item-3")

	tp.send('j', term.Enter, 'j', term.NewLine, 'x')
	if err := tp.stop(t); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(tp.selected) != "[item-2 item-3]" || fmt.Sprint(tp.handled) != "[item-3]" {
		t.Fatalf("got selected %v handled %v", tp.selected, tp.handled)
	}
}

func TestPromptWithoutVimKeys(t *testing.T) {
	tp := startPrompt(t, syncList(t, numbered(5), 3), &Options{LineSize: 3, VimKeys: false})
	tp.send('j', 'x')
	if err := tp.stop(t); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(tp.handled) != "[item-0]" {
		t.Fatalf("j moved the cursor without vim keys, handled %v", tp.handled)
	}
}

func TestPromptSearch(t *testing.T) {
	items := []string{"alpha", "beta", "gamma", "delta"}
	tp := startPrompt(t, syncList(t, items, 4), &Options{LineSize: 4})

	tp.send('/', 'm', 'm')
	frame := tp.waitFrame(t, "Search Items", "Search Items mm", "> gamma")
	if strings.Contains(frame, "alpha") {
		t.Fatalf("alpha does not match mm:\n%s", frame)
	}

	tp.send(term.Backspace, term.Backspace2, rune(term.KeyCtrlA), rune(term.KeyESC), 'l', 't')
	frame = tp.waitFrame(t, "Search Items", "Search Items lt", "> delta")
	if strings.Contains(frame, "\x01") || strings.Contains(frame, "\x1b") {
		t.Fatalf("control characters were added to the search: %q", frame)
	}

	tp.send(rune(term.KeyCtrlU), 'z', 'z', 'z')
	tp.waitFrame(t, "Search Items", "Search Items zzz", "Not found.")
	tp.send('x') // no item, no handler call

	tp.send(rune(term.KeyCtrlU), '/')
	tp.waitFrame(t, "Items", "> alpha", "delta")
	if err := tp.stop(t); err != nil {
		t.Fatal(err)
	}
	if len(tp.handled) != 0 {
		t.Fatalf("handler called without an item: %v", tp.handled)
	}
}

func TestPromptKeepsSearchTermAfterLeavingSearchMode(t *testing.T) {
	tp := startPrompt(t, syncList(t, []string{"alpha", "beta"}, 2), &Options{LineSize: 2})
	tp.send('/', 'b', '/')
	tp.waitFrame(t, "Items", "Items /b", "> beta")
}

func TestPromptHelp(t *testing.T) {
	tp := startPrompt(t, syncList(t, numbered(3), 3), &Options{LineSize: 3})
	tp.send('?')
	tp.waitFrame(t, "fail: ", "fail: e", "mark: x", "navigation: ← ↓ ↑ → (h,j,k,l)", "toggle search: /", "press any key to return.")
	tp.send('x') // leaves the help without running the binding
	tp.waitFrame(t, "Items", "> item-0", "info of item-0")
	if err := tp.stop(t); err != nil {
		t.Fatal(err)
	}
	if len(tp.handled) != 0 {
		t.Fatalf("key pressed to leave the help ran %v", tp.handled)
	}
}

func TestPromptShowsHandlerErrors(t *testing.T) {
	tp := startPrompt(t, syncList(t, numbered(3), 3), &Options{LineSize: 3})
	tp.send('e')
	frame := tp.waitFrame(t, "Items", "something failed", "hint: try again")
	if !strings.Contains(frame, "info of item-0") {
		t.Fatalf("error replaced the screen:\n%s", frame)
	}
	tp.send(term.ArrowDown)
	frame = tp.waitFrame(t, "Items", "> item-1")
	if strings.Contains(frame, "something failed") {
		t.Fatalf("error is still shown after the next key:\n%s", frame)
	}
	if err := tp.stop(t); err != nil {
		t.Fatalf("handler error ended the prompt: %v", err)
	}
}

func TestPromptStop(t *testing.T) {
	tp := startPrompt(t, syncList(t, numbered(3), 3), &Options{LineSize: 3})
	failure := errors.New("loading failed")
	go tp.Fail(failure)
	select {
	case err := <-tp.done:
		tp.done <- err
		if err != failure {
			t.Fatalf("got %v, want %v", err, failure)
		}
	case <-timeout():
		t.Fatal("Fail did not stop the prompt")
	}
	// stopping a stopped prompt must not block
	tp.Stop()
	tp.Fail(failure)
}

func TestPromptReadError(t *testing.T) {
	tp := startPrompt(t, syncList(t, numbered(3), 3), &Options{LineSize: 3})
	readErr := errors.New("stdin closed")
	tp.events <- keyEvent{err: readErr}
	select {
	case err := <-tp.done:
		tp.done <- err
		if err != readErr {
			t.Fatalf("got %v, want %v", err, readErr)
		}
	case <-timeout():
		t.Fatal("read error did not stop the prompt")
	}
}

func TestPromptRendersLoadedItems(t *testing.T) {
	ch := make(chan interface{})
	l, err := NewAsyncList(ch, 3)
	if err != nil {
		t.Fatal(err)
	}
	tp := startPrompt(t, l, &Options{LineSize: 3})
	tp.waitFrame(t, "Items", "Not found.")
	for _, item := range numbered(3) {
		ch <- item
	}
	close(ch)
	tp.waitFrame(t, "Items", "> item-0", "item-2", "info of item-0")
}

func TestPromptState(t *testing.T) {
	tp := startPrompt(t, syncList(t, numbered(10), 3), &Options{LineSize: 3})
	if err := tp.stop(t); err != nil {
		t.Fatal(err)
	}
	tp.list.SetCursor(5)
	state := tp.State()
	if state.Cursor != 5 || state.Scroll != 3 || state.ListSize != 3 || state.SearchLabel != "Items" || tp.ListSize() != 3 {
		t.Fatalf("got state %+v", state)
	}

	tp.SetState(&State{List: syncList(t, []string{"other"}, 3), SearchLabel: "Others", SearchMode: true, SearchStr: "o"})
	if tp.itemsLabel != "Others" || !tp.inputMode || tp.input != "o" {
		t.Fatalf("state was not applied: %+v", tp.Prompt)
	}
	tp.SetState(state)
	if items, active := tp.list.Items(); fmt.Sprint(items[active]) != "item-5" {
		t.Fatalf("restored state selects %v", items[active])
	}

	tp.SetExitMsg([][]term.Cell{term.Cprint("bye")})
	if len(tp.exitMsg) != 1 {
		t.Fatal("exit message was not set")
	}
}

func TestPromptSuspend(t *testing.T) {
	tp := startPrompt(t, syncList(t, numbered(3), 3), &Options{LineSize: 3})
	err := tp.AddKeyBinding(&KeyBinding{Key: 's', Display: "s", Desc: "suspend", Handler: func(item interface{}) error {
		return tp.Suspend(func() error {
			// e.g. a command printing its output
			_, _ = tp.out.Write([]byte("COMMAND OUTPUT\n"))
			return errors.New("command failed")
		})
	}})
	if err != nil {
		t.Fatal(err)
	}
	tp.waitFrame(t, "Items", "> item-0")
	tp.send('s')
	// the prompt is drawn again below the output, with the error of fn
	frame := tp.waitFrame(t, "COMMAND OUTPUT", "Items", "> item-0", "command failed")
	if !strings.HasPrefix(frame, "COMMAND OUTPUT") {
		t.Fatalf("the prompt was not drawn below the output:\n%s", frame)
	}
}

func TestNoticeLines(t *testing.T) {
	got := noticeLines("one\n\n  two  \nthree\nfour\nfive\nsix\n")
	if fmt.Sprint(got) != "[one two three four five]" {
		t.Fatalf("got %q", got)
	}
	if noticeLines("") != nil {
		t.Fatal("empty notice has lines")
	}
}

func TestItemText(t *testing.T) {
	text := func(cells [][]term.Cell) string {
		var s strings.Builder
		for _, c := range cells[0] {
			s.WriteRune(c.Ch)
			for _, a := range c.Attr {
				if a == color.Underline {
					s.WriteRune('_')
				}
			}
		}
		return s.String()
	}
	if got := text(itemText("plain", nil, false)); got != "  plain" {
		t.Fatalf("got %q", got)
	}
	// matches are byte offsets, the underlined characters are marked with _
	if got := text(itemText("Ünï ab", []int{len("Ünï "), len("Ünï a")}, true)); got != "> Ünï a_b_" {
		t.Fatalf("got %q", got)
	}
}

func TestRenderSearch(t *testing.T) {
	text := func(cells []term.Cell) string {
		var s strings.Builder
		for _, c := range cells {
			s.WriteRune(c.Ch)
		}
		return s.String()
	}
	for _, tt := range []struct {
		inputMode bool
		input     string
		want      string
	}{
		{false, "", "Files"},
		{false, "abc", "Files /abc"},
		{true, "abc", "Search Files abc█"},
	} {
		if got := text(renderSearch("Files", tt.inputMode, tt.input)); got != tt.want {
			t.Errorf("got %q, want %q", got, tt.want)
		}
	}
}
