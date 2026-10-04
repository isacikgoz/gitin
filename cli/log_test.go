package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/isacikgoz/gitin/git"
	"github.com/isacikgoz/gitin/internal/gittest"
)

func logRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.NewRepo(t)
	gittest.Commit(t, dir, "root", map[string]string{"root.txt": "r\n"})
	gittest.Commit(t, dir, "second", map[string]string{"root.txt": "r\ns\n", "second.txt": "2\n"})
	return dir
}

// newTestLog returns a log prompt after all commits are loaded
func newTestLog(t *testing.T, dir string) *log {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	l, err := newLog(ctx, open(t, dir), opts)
	if err != nil {
		t.Fatal(err)
	}
	want := len(commits(t, l.repository))
	deadline := time.Now().Add(10 * time.Second)
	for len(visible(l.prompt)) < min(want, opts.LineSize) {
		if time.Now().After(deadline) {
			t.Fatalf("commits were not loaded, got %v", visible(l.prompt))
		}
		time.Sleep(5 * time.Millisecond)
	}
	return l
}

func TestLogPrompt(t *testing.T) {
	p, err := LogPrompt(context.Background(), open(t, logRepo(t)), opts)
	if err != nil || p == nil {
		t.Fatalf("got prompt %v, error %v", p, err)
	}
}

func TestLogListsCommits(t *testing.T) {
	l := newTestLog(t, logRepo(t))
	if got := strings.Join(visible(l.prompt), ","); got != "second,root" {
		t.Fatalf("got %s", got)
	}
}

func TestLogFilesOfCommit(t *testing.T) {
	l := newTestLog(t, logRepo(t))
	second := selected(t, l.prompt).(*git.Commit)

	if err := l.onSelect(second); err != nil {
		t.Fatal(err)
	}
	state := l.prompt.State()
	if state.SearchLabel != "Files" || strings.Join(visible(l.prompt), ",") != "root.txt,second.txt" {
		t.Fatalf("got %s: %v", state.SearchLabel, visible(l.prompt))
	}

	out := captureTerminal(t)
	if err := l.onSelect(selected(t, l.prompt)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "diff --git a/root.txt b/root.txt") || !strings.Contains(out.String(), "+s") ||
		strings.Contains(out.String(), "second.txt") {
		t.Fatalf("got diff %q", out.String())
	}

	// q goes back to the commits
	if err := l.quit(selected(t, l.prompt)); err != nil {
		t.Fatal(err)
	}
	if got := l.prompt.State().SearchLabel; got != "Commits" {
		t.Fatalf("got %s after quitting the files", got)
	}
}

func TestLogFilesOfRootCommit(t *testing.T) {
	l := newTestLog(t, logRepo(t))
	l.prompt.State().List.Next()
	root := selected(t, l.prompt).(*git.Commit)
	if err := l.onSelect(root); err != nil {
		t.Fatal(err)
	}
	out := captureTerminal(t)
	if err := l.onSelect(selected(t, l.prompt)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "new file mode") || !strings.Contains(out.String(), "+r") {
		t.Fatalf("got diff %q", out.String())
	}
}

func TestLogEmptyCommit(t *testing.T) {
	dir := logRepo(t)
	gittest.Commit(t, dir, "empty", nil)
	l := newTestLog(t, dir)
	if err := l.onSelect(selected(t, l.prompt)); err != nil {
		t.Fatal(err)
	}
	if got := l.prompt.State().SearchLabel; got != "Commits" {
		t.Fatalf("got %s for a commit without files", got)
	}
}

func TestLogShowCommit(t *testing.T) {
	l := newTestLog(t, logRepo(t))
	c := selected(t, l.prompt)

	out := captureTerminal(t)
	if err := l.commitStat(c); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "second.txt | 1 +") || strings.Contains(out.String(), "+2") {
		t.Fatalf("got stat %q", out.String())
	}

	out = captureTerminal(t)
	if err := l.commitDiff(c); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "+2") {
		t.Fatalf("got diff %q", out.String())
	}

	// only commits have stats and diffs
	out = captureTerminal(t)
	for _, f := range []func(interface{}) error{l.commitStat, l.commitDiff} {
		if err := f(&git.DiffDelta{}); err != nil || out.Len() != 0 {
			t.Fatalf("got %v and output %q for a file", err, out.String())
		}
	}
}

func TestLogShowsDiffErrors(t *testing.T) {
	dir := logRepo(t)
	l := newTestLog(t, dir)
	captureTerminal(t)
	c := selected(t, l.prompt).(*git.Commit)
	gone := &git.Commit{Hash: "1111111111111111111111111111111111111111", Parents: c.Parents}
	if err := l.commitStat(gone); err == nil || !strings.Contains(err.Error(), "bad object") {
		t.Fatalf("got error %v", err)
	}
}
