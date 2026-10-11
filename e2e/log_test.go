package e2e

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/isacikgoz/gitin/internal/gittest"
)

func logRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.NewRepo(t)
	gittest.Commit(t, dir, "root commit", map[string]string{"root.txt": "r\n"})
	gittest.Git(t, dir, "tag", "--annotate", "--message=first release", "v1.0")
	gittest.Commit(t, dir, "Speed up by 50%", map[string]string{"root.txt": "r\nfast\n", "50% off.txt": "sale\n"})
	gittest.Git(t, dir, "tag", "light")
	return dir
}

func TestLogShowsCommitsWithRefs(t *testing.T) {
	s := start(t, logRepo(t), []string{"log"})
	screen := s.waitScreen("Speed up by 50%", "root commit", "Ada Lovelace <ada@example.com>", "(HEAD -> main, tag: light)")
	if strings.Contains(screen, "NOVERB") {
		t.Fatalf("%% is not shown as is:\n%s", screen)
	}
	s.matchScreen("commits")

	s.send(down)
	// annotated tags are shown on the commit they point to
	s.waitSelected("root commit")
	s.waitScreen("(tag: v1.0)")

	s.send("q")
	if code := s.wait(); code != 0 {
		t.Fatalf("got exit code %d", code)
	}
}

func TestLogFilesAndDiffs(t *testing.T) {
	s := start(t, logRepo(t), []string{"log"})
	s.waitScreen("Speed up by 50%")

	s.send(enter)
	s.waitScreen("[A] 50% off.txt", "[M] root.txt", "1 addition.")
	s.matchScreen("files")

	s.send(down, enter)
	s.waitOutput("diff --git a/root.txt b/root.txt", "+fast")

	// q goes back from the files to the commits, then quits
	s.send("q")
	s.waitScreen("Commits", "Speed up by 50%")
	s.send("q")
	if code := s.wait(); code != 0 {
		t.Fatalf("got exit code %d", code)
	}
}

func TestLogFileOfRootCommit(t *testing.T) {
	s := start(t, logRepo(t), []string{"log"})
	s.waitScreen("root commit")
	s.send(down, enter)
	s.waitScreen("[A] root.txt")
	s.send(enter)
	s.waitOutput("new file mode", "+++ b/root.txt", "+r")
}

func TestLogCommitStatAndDiff(t *testing.T) {
	s := start(t, logRepo(t), []string{"log"})
	s.waitScreen("Speed up by 50%")
	s.send("s")
	s.waitOutput("2 files changed")
	s.send("d")
	s.waitOutput("+sale")
}

func TestLogSearch(t *testing.T) {
	dir := gittest.NewRepo(t)
	for _, msg := range []string{"add parser", "fix typo", "add renderer", "update docs"} {
		gittest.Commit(t, dir, msg, nil)
	}
	s := start(t, dir, []string{"log"})
	s.waitScreen("update docs")

	s.send("/", "a", "d", "d")
	screen := s.waitScreen("Search Commits add", "add renderer", "add parser")
	if strings.Contains(screen, "fix typo") || strings.Contains(screen, "update docs") {
		t.Fatalf("search shows commits that don't match:\n%s", screen)
	}
	s.matchScreen("search")

	s.send(backspace, backspace, backspace, "t", "y", "p", "o")
	screen = s.waitScreen("Search Commits typo", "fix typo")
	if strings.Contains(screen, "add parser") {
		t.Fatalf("results of the previous search are shown:\n%s", screen)
	}

	s.send(ctrlU, "/")
	s.waitScreen("add parser", "fix typo", "update docs")
}

func TestLogShallowClone(t *testing.T) {
	dir := gittest.NewRepo(t)
	for i := range 5 {
		gittest.Commit(t, dir, fmt.Sprintf("commit %d", i), map[string]string{"a": fmt.Sprint(i)})
	}
	clone := gittest.Clone(t, dir, "--depth=2")
	s := start(t, clone, []string{"log"})
	s.waitScreen("commit 4", "commit 3")
	if !s.running() {
		t.Fatal("gitin exited in a shallow clone")
	}
	s.send("q")
	if code := s.wait(); code != 0 {
		t.Fatalf("got exit code %d", code)
	}
}

func TestLogStaysResponsiveWhileLoading(t *testing.T) {
	dir := gittest.NewRepo(t)
	var script strings.Builder
	for i := 1; i <= 50000; i++ {
		fmt.Fprintf(&script, "commit refs/heads/main\nmark :%d\ncommitter A <a@b> %d +0000\ndata 6\nc%05d", i, 1000000+i, i)
		if i > 1 {
			fmt.Fprintf(&script, "\nfrom :%d", i-1)
		}
		script.WriteString("\n\n")
	}
	cmd := exec.Command("git", "fast-import", "--quiet")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(script.String())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fast-import: %v\n%s", err, out)
	}

	// typing while the history is loaded used to freeze gitin
	s := start(t, dir, []string{"log"})
	s.send("/", "c", "4", "9", "9", "9", "9")
	s.waitSelected("c49999")
	s.send(ctrlC)
	if code := s.wait(); code != 0 {
		t.Fatalf("got exit code %d", code)
	}
}

func TestLogWithoutCommits(t *testing.T) {
	s := start(t, gittest.NewRepo(t), []string{"log"})
	if code := s.wait(); code != 1 {
		t.Fatalf("got exit code %d, want 1", code)
	}
	if !strings.Contains(s.output(), "does not have any commits yet") {
		t.Fatalf("got output %q", s.output())
	}
}

func TestUnknownKeysDoNotQuit(t *testing.T) {
	dir := gittest.NewRepo(t)
	gittest.Commit(t, dir, "first", nil)
	gittest.Commit(t, dir, "second", nil)
	s := start(t, dir, []string{"log"})
	s.waitScreen("second")

	// Alt+b, F5, ctrl+right, then down in application cursor mode
	s.send("\x1bb", "\x1b[15~", "\x1b[1;5C", "\x1bOB")
	s.waitSelected("first")
	if !s.running() {
		t.Fatalf("gitin exited: %s", s.output())
	}
}

func TestHelp(t *testing.T) {
	dir := gittest.NewRepo(t)
	gittest.Commit(t, dir, "first", nil)
	s := start(t, dir, []string{"log"})
	s.waitScreen("first")
	s.send("?")
	s.waitScreen("show stat: s", "show diff: d", "toggle search: /", "press any key to return.")
	s.matchScreen("help")
	s.send("x")
	s.waitUntil("the commits", func(screen string) bool {
		return strings.Contains(screen, "first") && !strings.Contains(screen, "press any key to return.")
	})
}
