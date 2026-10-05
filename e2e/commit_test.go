package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isacikgoz/gitin/internal/gittest"
)

// commitScreen starts the prompt asking about the commit checks
const commitScreen = "\nCommit\n"

// commitRepo has commit checks and app.txt with a staged and an unstaged change
func commitRepo(t *testing.T, checks string) string {
	t.Helper()
	dir := gittest.NewRepo(t)
	gittest.Commit(t, dir, "base", map[string]string{".gitin.yml": "commit:\n  checks:\n" + checks, "app.txt": "one\n"})
	gittest.WriteFile(t, dir, "app.txt", "one\nstaged\n")
	gittest.Git(t, dir, "add", "app.txt")
	gittest.WriteFile(t, dir, "app.txt", "one\nstaged\nunstaged\n")
	return dir
}

// the check passes if the working tree has only the staged changes
const stagedOnlyCheck = `    - name: Staged only
      run: git diff --quiet && echo STAGED ONLY
`

func lastCommit(t *testing.T, dir string) string {
	t.Helper()
	return gittest.Git(t, dir, "log", "-1", "--format=%s")
}

func assertRestored(t *testing.T, dir string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "app.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "one\nstaged\nunstaged\n" {
		t.Fatalf("app.txt has %q, the unstaged change was not restored", data)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "gitin-unstaged.patch")); !os.IsNotExist(err) {
		t.Fatal("the unstaged changes are still set aside")
	}
}

func TestCommitRunsChecksOnStagedChanges(t *testing.T) {
	dir := commitRepo(t, stagedOnlyCheck)
	s := start(t, dir, []string{"status"}, editor(t, "checked commit"))
	s.waitFrame("Files", 0, "app.txt")
	m := s.mark()
	s.send("c")
	s.waitFrame(commitScreen, m, "> Run checks, then commit", "Commit without checks", "Checks of .gitin.yml: Staged only",
		"Unstaged changes are set aside while they run, they check what gets committed.")
	m = s.mark()
	s.send(enter)
	s.waitFrame("Files", m, "[M] app.txt")
	out := s.text()[m:]
	for _, want := range []string{"Unstaged changes are set aside until the checks finish.", "STAGED ONLY", "✔ Staged only passed in", "1 file changed"} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not show %q", want)
		}
	}
	if got := lastCommit(t, dir); got != "checked commit" {
		t.Fatalf("got last commit %q", got)
	}
	if got := gittest.Git(t, dir, "show", "HEAD:app.txt"); got != "one\nstaged" {
		t.Fatalf("committed %q, want only the staged change", got)
	}
	assertRestored(t, dir)
}

func TestCommitWithoutChecks(t *testing.T) {
	dir := commitRepo(t, stagedOnlyCheck)
	s := start(t, dir, []string{"status"}, editor(t, "unchecked commit"))
	s.waitFrame("Files", 0, "app.txt")
	m := s.mark()
	s.send("c")
	s.waitFrame(commitScreen, m, "> Run checks, then commit")
	m = s.mark()
	s.send(down, enter)
	s.waitFrame("Files", m, "[M] app.txt")
	if strings.Contains(s.text(), "STAGED ONLY") {
		t.Fatal("the checks ran")
	}
	if got := lastCommit(t, dir); got != "unchecked commit" {
		t.Fatalf("got last commit %q", got)
	}
	assertRestored(t, dir)
}

func TestCommitCancel(t *testing.T) {
	dir := commitRepo(t, stagedOnlyCheck)
	s := start(t, dir, []string{"status"}, editor(t, "cancelled"))
	s.waitFrame("Files", 0, "app.txt")
	m := s.mark()
	s.send("c")
	s.waitFrame(commitScreen, m, "> Run checks, then commit")
	m = s.mark()
	s.send("q")
	s.waitFrame("Files", m, "> [M] app.txt")
	if got := lastCommit(t, dir); got != "base" || strings.Contains(s.text(), "STAGED ONLY") {
		t.Fatalf("got last commit %q", got)
	}
	if !s.running() {
		t.Fatal("q left gitin instead of the commit screen")
	}
}

func TestCommitFailingCheck(t *testing.T) {
	const failing = "    - name: Lint\n      run: |\n        echo LINT ERROR >&2\n        exit 3\n"

	t.Run("cancel", func(t *testing.T) {
		dir := commitRepo(t, failing)
		s := start(t, dir, []string{"status"}, editor(t, "failed"))
		s.waitFrame("Files", 0, "app.txt")
		s.send("c")
		s.waitFrame(commitScreen, 0, "> Run checks, then commit")
		m := s.mark()
		s.send(enter)
		s.waitFrame(commitScreen, m, "> Cancel", "Commit anyway", "Lint failed (exit status 3), nothing was committed yet.")
		m = s.mark()
		s.send(enter)
		s.waitFrame("Files", m, "app.txt", "Lint failed, nothing was committed")
		if got := lastCommit(t, dir); got != "base" {
			t.Fatalf("got last commit %q", got)
		}
		assertRestored(t, dir)
	})

	t.Run("commit anyway", func(t *testing.T) {
		dir := commitRepo(t, failing)
		s := start(t, dir, []string{"status"}, editor(t, "committed anyway"))
		s.waitFrame("Files", 0, "app.txt")
		s.send("c")
		s.waitFrame(commitScreen, 0, "> Run checks, then commit")
		m := s.mark()
		s.send(enter)
		s.waitFrame(commitScreen, m, "> Cancel")
		m = s.mark()
		s.send(down, enter)
		s.waitFrame("Files", m, "[M] app.txt")
		if got := lastCommit(t, dir); got != "committed anyway" {
			t.Fatalf("got last commit %q", got)
		}
		assertRestored(t, dir)
	})
}

func TestCommitStopCheck(t *testing.T) {
	// the header shows the command, the output must differ from it
	dir := commitRepo(t, "    - name: Slow\n      run: printf 'CHECK %s\\n' STARTED; sleep 30\n")
	s := start(t, dir, []string{"status"}, editor(t, "stopped"))
	s.waitFrame("Files", 0, "app.txt")
	s.send("c")
	s.waitFrame(commitScreen, 0, "> Run checks, then commit")
	m := s.mark()
	s.send(enter)
	s.waitText(m, "CHECK STARTED")
	m = s.mark()
	s.send(ctrlC)
	s.waitFrame("Files", m, "app.txt", "the checks were stopped, nothing was committed")
	if got := lastCommit(t, dir); got != "base" {
		t.Fatalf("got last commit %q", got)
	}
	assertRestored(t, dir)
}

// A formatter rewrites a file the user changed without staging, the user's
// version wins
func TestCommitCheckRewritesFile(t *testing.T) {
	dir := commitRepo(t, "    - name: Format\n      run: echo formatted > app.txt\n")
	s := start(t, dir, []string{"status"}, editor(t, "formatted"))
	s.waitFrame("Files", 0, "app.txt")
	s.send("c")
	s.waitFrame(commitScreen, 0, "> Run checks, then commit")
	m := s.mark()
	s.send(enter)
	s.waitFrame("Files", m, "app.txt")
	if !strings.Contains(s.text()[m:], "The checks changed files with unstaged changes, their changes were undone to restore yours.") {
		t.Fatalf("got output %q", s.text()[m:])
	}
	if got := lastCommit(t, dir); got != "formatted" {
		t.Fatalf("got last commit %q", got)
	}
	assertRestored(t, dir)
}

func TestAmendRunsChecks(t *testing.T) {
	dir := commitRepo(t, stagedOnlyCheck)
	s := start(t, dir, []string{"status"}, editor(t, "amended base"))
	s.waitFrame("Files", 0, "app.txt")
	m := s.mark()
	s.send("m")
	s.waitFrame("\nAmend\n", m, "> Run checks, then amend", "Amend without checks")
	m = s.mark()
	s.send(enter)
	s.waitFrame("Files", m, "app.txt")
	if !strings.Contains(s.text()[m:], "STAGED ONLY") {
		t.Fatal("the checks did not run")
	}
	if got := gittest.Git(t, dir, "log", "--format=%s"); got != "amended base" {
		t.Fatalf("got commits %q", got)
	}
	assertRestored(t, dir)
}

func TestCommitChecksNeedStagedChanges(t *testing.T) {
	dir := commitRepo(t, stagedOnlyCheck)
	gittest.Git(t, dir, "reset", "--quiet")
	s := start(t, dir, []string{"status"}, editor(t, "nothing"))
	s.waitFrame("Files", 0, "app.txt")
	m := s.mark()
	s.send("c")
	s.waitFrame("Files", m, "nothing to commit, stage changes with space or a first")
	if strings.Contains(s.text(), "Run checks") {
		t.Fatal("asked to run checks without anything to commit")
	}
}

func TestCommitWithLeftOverChanges(t *testing.T) {
	dir := commitRepo(t, stagedOnlyCheck)
	gittest.WriteFile(t, dir, ".git/gitin-unstaged.patch", "changes of a run that crashed")
	s := start(t, dir, []string{"status"}, editor(t, "blocked"))
	s.waitFrame("Files", 0, "app.txt")
	s.send("c")
	s.waitFrame(commitScreen, 0, "> Run checks, then commit")
	m := s.mark()
	s.send(enter)
	s.waitFrame("Files", m, "unstaged changes set aside earlier are still in")
	if got := lastCommit(t, dir); got != "base" {
		t.Fatalf("got last commit %q", got)
	}
}
