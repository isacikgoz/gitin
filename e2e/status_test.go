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
	s.waitScreen("[D] deleted.txt", "[M] modified.txt", "[U] new.txt", "On branch main",
		"Your branch is not tracking a remote branch.")
	s.matchScreen("files")
	s.send("q")
	if code := s.wait(); code != 0 {
		t.Fatalf("got exit code %d", code)
	}
}

func TestStatusDiffs(t *testing.T) {
	s := start(t, statusRepo(t), []string{"status"})
	s.waitScreen("deleted.txt")

	// the staged deletion
	s.send(enter)
	s.waitOutput("deleted file mode")

	s.send(down, enter)
	s.waitOutput("+one", "+ten")

	// an untracked file is shown as a new file
	s.send(down, enter)
	s.waitOutput("+brand")
	if !s.running() {
		t.Fatal("gitin exited")
	}
}

func TestStatusStageAndUnstage(t *testing.T) {
	dir := statusRepo(t)
	s := start(t, dir, []string{"status"})
	s.waitScreen("deleted.txt")

	s.send(" ") // unstage the deletion
	s.waitScreen("> [D] deleted.txt")
	if got := shortStatus(t, dir); !strings.Contains(got, " D deleted.txt") {
		t.Fatalf("got\n%s", got)
	}

	s.send("a")
	s.waitScreen("[A] new.txt")
	if got := shortStatus(t, dir); got != "D  deleted.txt\nM  modified.txt\nA  new.txt" {
		t.Fatalf("after add all got\n%s", got)
	}

	s.send("r")
	s.waitScreen("[U] new.txt")
	if got := shortStatus(t, dir); got != " D deleted.txt\n M modified.txt\n?? new.txt" {
		t.Fatalf("after reset all got\n%s", got)
	}
}

func TestStatusDiscard(t *testing.T) {
	dir := statusRepo(t)
	s := start(t, dir, []string{"status"})
	s.waitScreen("deleted.txt")

	s.send("!")
	s.waitScreen("staged changes are not discarded, press space to unstage them first")
	s.matchScreen("staged changes")

	s.send(down, down)
	s.waitSelected("new.txt")
	s.send("!")
	s.waitUntil("new.txt removed", func(screen string) bool {
		return strings.Contains(screen, "modified.txt") && !strings.Contains(screen, "new.txt")
	})
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
	s.waitScreen("modified.txt")

	// stage the first of the two hunks of modified.txt
	s.send(down, "p")
	s.waitScreen("+one", "+ten")
	s.matchScreen("hunks")
	s.send(" ", "q")
	// listed twice, staged and not staged
	s.waitUntil("modified.txt twice", func(screen string) bool {
		return strings.Count(screen, "[M] modified.txt") == 2
	})
	s.matchScreen("partly staged")
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
	s.send(down, down, "p")
	s.waitScreen("+brand")
	s.send(" ", "q")
	s.waitScreen("[A] new.txt")
}

func TestStatusCommit(t *testing.T) {
	dir := statusRepo(t)
	gittest.Git(t, dir, "add", "--all")
	s := start(t, dir, []string{"status"}, editor(t, "commit from gitin"))
	s.waitScreen("deleted.txt")
	s.send("c")
	// the working tree is clean afterwards, gitin says so and exits
	if code := s.wait(); code != 0 {
		t.Fatalf("got exit code %d", code)
	}
	if !strings.Contains(s.output(), "3 files changed") || !strings.Contains(s.output(), "Nothing to commit, working tree clean") {
		t.Fatalf("got output %q", s.output())
	}
	if got := gittest.Git(t, dir, "log", "-1", "--format=%s"); got != "commit from gitin" {
		t.Fatalf("got last commit %q", got)
	}
}

func TestStatusAmend(t *testing.T) {
	dir := statusRepo(t)
	s := start(t, dir, []string{"status"}, editor(t, "amended base"))
	s.waitScreen("deleted.txt")
	s.send("m")
	s.waitScreen("modified.txt")
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
	s.waitScreen("deleted.txt")
	s.send("c")
	s.waitScreen("HOOK REJECTED: fix lint first")
	s.matchScreen("rejected")
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
	s.waitScreen("deleted.txt", "sub/")
	s.send("a") // paths are relative to the root of the working tree
	s.waitScreen("[A] sub/dir/file.txt")
}

func TestStatusDetachedHead(t *testing.T) {
	dir := statusRepo(t)
	gittest.Git(t, dir, "checkout", "--quiet", "--detach")
	s := start(t, dir, []string{"status"})
	s.waitScreen("HEAD detached at ")
}

func TestStatusAheadOfUpstream(t *testing.T) {
	upstream := gittest.NewRepo(t)
	gittest.Commit(t, upstream, "base", map[string]string{"a": "1"})
	dir := gittest.Clone(t, upstream)
	gittest.Commit(t, dir, "local", map[string]string{"a": "2"})
	gittest.WriteFile(t, dir, "a", "3")
	s := start(t, dir, []string{"status"})
	s.waitScreen("Your branch is ahead of origin/main by 1 commit(s).")
}
