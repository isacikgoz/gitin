package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isacikgoz/gitin/internal/gittest"
)

func statusRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.NewRepo(t)
	gittest.Commit(t, dir, "base", map[string]string{"modified.txt": "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n", "deleted.txt": "x\n"})
	gittest.WriteFile(t, dir, "modified.txt", "one\n2\n3\n4\n5\n6\n7\n8\n9\nten\n")
	gittest.Git(t, dir, "rm", "--quiet", "deleted.txt")
	gittest.WriteFile(t, dir, "new.txt", "brand\nnew\n")
	return dir
}

// shortStatus returns "git status --short" of dir
func shortStatus(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "status", "--short")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimRight(string(out), "\n")
}

func editor(t *testing.T, message string) string {
	t.Helper()
	script := filepath.Join(t.TempDir(), "editor")
	content := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' %q > \"$1\"\n", message)
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return "GIT_EDITOR=" + script
}

func TestStatusShowsEntries(t *testing.T) {
	s := start(t, statusRepo(t), []string{"status"})
	s.waitFrame("Files", 0, "[D] deleted.txt", "[M] modified.txt", "[U] new.txt", "On branch main",
		"Your branch is not tracking a remote branch.")
	s.send("q")
	if code := s.wait(); code != 0 {
		t.Fatalf("got exit code %d", code)
	}
}

func TestStatusDiffs(t *testing.T) {
	s := start(t, statusRepo(t), []string{"status"})
	s.waitFrame("Files", 0, "deleted.txt")

	// the staged deletion
	m := s.mark()
	s.send(enter)
	s.waitText(m, "deleted file mode")

	m = s.mark()
	s.send(down, enter)
	s.waitText(m, "+one", "+ten")

	// an untracked file is shown as a new file
	m = s.mark()
	s.send(down, enter)
	s.waitText(m, "+brand")
	if !s.running() {
		t.Fatal("gitin exited")
	}
}

func TestStatusStageAndUnstage(t *testing.T) {
	dir := statusRepo(t)
	s := start(t, dir, []string{"status"})
	s.waitFrame("Files", 0, "deleted.txt")

	m := s.mark()
	s.send(" ") // unstage the deletion
	s.waitFrame("Files", m, "> [D] deleted.txt")
	if got := shortStatus(t, dir); !strings.Contains(got, " D deleted.txt") {
		t.Fatalf("got\n%s", got)
	}

	m = s.mark()
	s.send("a")
	s.waitFrame("Files", m, "[A] new.txt")
	if got := shortStatus(t, dir); got != "D  deleted.txt\nM  modified.txt\nA  new.txt" {
		t.Fatalf("after add all got\n%s", got)
	}

	m = s.mark()
	s.send("r")
	s.waitFrame("Files", m, "[U] new.txt")
	if got := shortStatus(t, dir); got != " D deleted.txt\n M modified.txt\n?? new.txt" {
		t.Fatalf("after reset all got\n%s", got)
	}
}

func TestStatusDiscard(t *testing.T) {
	dir := statusRepo(t)
	s := start(t, dir, []string{"status"})
	s.waitFrame("Files", 0, "deleted.txt")

	m := s.mark()
	s.send("!")
	s.waitFrame("Files", m, "staged changes are not discarded, press space to unstage them first")

	m = s.mark()
	s.send(down, down, "!")
	s.waitFrame("Files", m, "deleted.txt", "modified.txt")
	if _, err := os.Stat(filepath.Join(dir, "new.txt")); !os.IsNotExist(err) {
		t.Fatal("untracked file was not removed")
	}
	if got := shortStatus(t, dir); got != "D  deleted.txt\n M modified.txt" {
		t.Fatalf("got\n%s", got)
	}
}

func TestStatusHunkStaging(t *testing.T) {
	dir := statusRepo(t)
	s := start(t, dir, []string{"status"})
	s.waitFrame("Files", 0, "modified.txt")

	// stage the first of the two hunks of modified.txt
	m := s.mark()
	s.send(down, "p")
	s.waitText(m, "+one", "+ten")
	m = s.mark()
	s.send(" ", "q")
	s.waitFrame("Files", m, "modified.txt")
	if got := shortStatus(t, dir); !strings.Contains(got, "MM modified.txt") {
		t.Fatalf("got\n%s", got)
	}
	cmd := exec.Command("git", "diff", "--cached", "--", "modified.txt")
	cmd.Dir = dir
	staged, _ := cmd.Output()
	if !strings.Contains(string(staged), "+one") || strings.Contains(string(staged), "+ten") {
		t.Fatalf("got staged changes\n%s", staged)
	}

	// a new file has a single hunk
	m = s.mark()
	s.send(down, down, "p")
	s.waitText(m, "+brand")
	m = s.mark()
	s.send(" ", "q")
	s.waitFrame("Files", m, "[A] new.txt")
}

func TestStatusCommit(t *testing.T) {
	dir := statusRepo(t)
	gittest.Git(t, dir, "add", "--all")
	s := start(t, dir, []string{"status"}, editor(t, "commit from gitin"))
	s.waitFrame("Files", 0, "deleted.txt")
	s.send("c")
	// the working tree is clean afterwards, gitin says so and exits
	if code := s.wait(); code != 0 {
		t.Fatalf("got exit code %d", code)
	}
	if !strings.Contains(s.text(), "3 files changed") || !strings.Contains(s.text(), "Nothing to commit, working tree clean") {
		t.Fatalf("got output %q", s.text())
	}
	if got := gittest.Git(t, dir, "log", "-1", "--format=%s"); got != "commit from gitin" {
		t.Fatalf("got last commit %q", got)
	}
}

func TestStatusAmend(t *testing.T) {
	dir := statusRepo(t)
	s := start(t, dir, []string{"status"}, editor(t, "amended base"))
	s.waitFrame("Files", 0, "deleted.txt")
	m := s.mark()
	s.send("m")
	s.waitFrame("Files", m, "modified.txt")
	if got := gittest.Git(t, dir, "log", "--format=%s"); got != "amended base" {
		t.Fatalf("got commits %q", got)
	}
}

func TestStatusShowsRejectedCommit(t *testing.T) {
	dir := statusRepo(t)
	hook := filepath.Join(dir, ".git", "hooks", "pre-commit")
	gittest.WriteFile(t, dir, ".git/hooks/pre-commit", "#!/bin/sh\necho 'HOOK REJECTED: fix lint first' >&2\nexit 1\n")
	if err := os.Chmod(hook, 0o755); err != nil {
		t.Fatal(err)
	}
	s := start(t, dir, []string{"status"}, editor(t, "rejected"))
	s.waitFrame("Files", 0, "deleted.txt")
	m := s.mark()
	s.send("c")
	s.waitFrame("Files", m, "HOOK REJECTED: fix lint first")
	if got := gittest.Git(t, dir, "log", "--format=%s"); got != "base" {
		t.Fatalf("got commits %q", got)
	}
}

func TestStatusOfCleanWorkingTree(t *testing.T) {
	dir := gittest.NewRepo(t)
	gittest.Commit(t, dir, "base", map[string]string{"a": "1"})
	out, code := runGitin(t, dir, "status")
	if code != 0 || !strings.Contains(out, "On branch main") || !strings.Contains(out, "Nothing to commit, working tree clean") {
		t.Fatalf("got exit code %d and %q", code, out)
	}
}

func TestStatusFromSubdirectory(t *testing.T) {
	dir := statusRepo(t)
	gittest.WriteFile(t, dir, "sub/dir/file.txt", "x")
	s := start(t, filepath.Join(dir, "sub", "dir"), []string{"status"})
	s.waitFrame("Files", 0, "deleted.txt", "sub/")
	m := s.mark()
	s.send("a") // paths are relative to the root of the working tree
	s.waitFrame("Files", m, "[A] sub/dir/file.txt")
}

func TestStatusDetachedHead(t *testing.T) {
	dir := statusRepo(t)
	gittest.Git(t, dir, "checkout", "--quiet", "--detach")
	s := start(t, dir, []string{"status"})
	s.waitFrame("Files", 0, "HEAD detached at ")
}

func TestStatusAheadOfUpstream(t *testing.T) {
	upstream := gittest.NewRepo(t)
	gittest.Commit(t, upstream, "base", map[string]string{"a": "1"})
	dir := gittest.Clone(t, upstream)
	gittest.Commit(t, dir, "local", map[string]string{"a": "2"})
	gittest.WriteFile(t, dir, "a", "3")
	s := start(t, dir, []string{"status"})
	s.waitFrame("Files", 0, "Your branch is ahead of origin/main by 1 commit(s).")
}
