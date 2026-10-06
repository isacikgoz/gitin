package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isacikgoz/gitin/internal/gittest"
)

func read(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if os.IsNotExist(err) {
		return "<deleted>"
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func shortStatus(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "status", "--short", "--untracked-files=all")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimRight(string(out), "\n")
}

// unstagedRepo has staged and unstaged changes in the same file, unstaged
// modifications, a deletion, a mode change and a binary file, an
// intent-to-add file, an untracked file and a file named like a pathspec
func unstagedRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.NewRepo(t)
	gittest.Commit(t, dir, "base", map[string]string{
		"both.txt": "1\n", "modified.txt": "1\n", "deleted.txt": "1\n", "script.sh": "echo\n", "bin.dat": "\x00\x01", "*.txt": "glob\n",
	})
	gittest.WriteFile(t, dir, "both.txt", "staged\n")
	gittest.Git(t, dir, "add", "both.txt")
	gittest.WriteFile(t, dir, "both.txt", "staged\nunstaged\n")
	gittest.WriteFile(t, dir, "modified.txt", "1\n2\n")
	gittest.WriteFile(t, dir, "bin.dat", "\x00\x02\x03")
	gittest.WriteFile(t, dir, "*.txt", "glob changed\n")
	if err := os.Remove(filepath.Join(dir, "deleted.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "script.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	gittest.WriteFile(t, dir, "intent.txt", "intent to add\n")
	gittest.Git(t, dir, "add", "--intent-to-add", "intent.txt")
	gittest.WriteFile(t, dir, "untracked.txt", "untracked\n")
	return dir
}

func TestSetAsideUnstaged(t *testing.T) {
	dir := unstagedRepo(t)
	before := shortStatus(t, dir)
	files := []string{"both.txt", "modified.txt", "deleted.txt", "bin.dat", "*.txt", "intent.txt", "untracked.txt"}
	contents := map[string]string{}
	for _, name := range files {
		contents[name] = read(t, dir, name)
	}

	u, err := open(t, dir).SetAsideUnstaged()
	if err != nil {
		t.Fatal(err)
	}
	if !u.Any() {
		t.Fatal("no unstaged changes were found")
	}
	// the working tree has what the next commit has, except for files git
	// doesn't track yet
	want := "M  both.txt\n A intent.txt\n?? untracked.txt"
	if got := shortStatus(t, dir); got != want {
		t.Fatalf("while set aside: got status\n%s\nwant\n%s", got, want)
	}
	for name, want := range map[string]string{"both.txt": "staged\n", "deleted.txt": "1\n", "*.txt": "glob\n", "intent.txt": "intent to add\n"} {
		if got := read(t, dir, name); got != want {
			t.Errorf("while set aside: %s has %q, want %q", name, got, want)
		}
	}

	undone, err := u.Restore()
	if err != nil || undone {
		t.Fatalf("got undone %v, error %v", undone, err)
	}
	if got := shortStatus(t, dir); got != before {
		t.Fatalf("after restoring: got status\n%s\nwant\n%s", got, before)
	}
	for name, want := range contents {
		if got := read(t, dir, name); got != want {
			t.Errorf("after restoring: %s has %q, want %q", name, got, want)
		}
	}
	if info, err := os.Stat(filepath.Join(dir, "script.sh")); err != nil || info.Mode()&0o100 == 0 {
		t.Errorf("after restoring: script.sh is not executable")
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", unstagedPatch)); !os.IsNotExist(err) {
		t.Error("the patch was not removed")
	}
}

func TestSetAsideWithoutUnstagedChanges(t *testing.T) {
	dir := gittest.NewRepo(t)
	gittest.Commit(t, dir, "base", map[string]string{"a": "1"})
	gittest.WriteFile(t, dir, "a", "2")
	gittest.Git(t, dir, "add", "a")
	gittest.WriteFile(t, dir, "new", "untracked")

	u, err := open(t, dir).SetAsideUnstaged()
	if err != nil || u.Any() {
		t.Fatalf("got %+v, %v", u, err)
	}
	if undone, err := u.Restore(); err != nil || undone {
		t.Fatalf("got undone %v, error %v", undone, err)
	}
	if got := shortStatus(t, dir); got != "M  a\n?? new" {
		t.Fatalf("got status %q", got)
	}
}

func TestRestoreAfterChanges(t *testing.T) {
	t.Run("other files", func(t *testing.T) {
		dir := unstagedRepo(t)
		u, err := open(t, dir).SetAsideUnstaged()
		if err != nil {
			t.Fatal(err)
		}
		// e.g. a check writes a report or formats a file without unstaged changes
		gittest.WriteFile(t, dir, "report.txt", "report\n")
		gittest.WriteFile(t, dir, "script.sh", "echo formatted\n")
		if undone, err := u.Restore(); err != nil || undone {
			t.Fatalf("got undone %v, error %v", undone, err)
		}
		if read(t, dir, "script.sh") != "echo formatted\n" || read(t, dir, "report.txt") != "report\n" || read(t, dir, "modified.txt") != "1\n2\n" {
			t.Fatal("the changes were not kept")
		}
	})

	t.Run("same files", func(t *testing.T) {
		dir := unstagedRepo(t)
		u, err := open(t, dir).SetAsideUnstaged()
		if err != nil {
			t.Fatal(err)
		}
		// a formatter rewrites a file the unstaged changes touch
		gittest.WriteFile(t, dir, "modified.txt", "formatted\n")
		undone, err := u.Restore()
		if err != nil || !undone {
			t.Fatalf("got undone %v, error %v", undone, err)
		}
		if got := read(t, dir, "modified.txt"); got != "1\n2\n" {
			t.Fatalf("got %q, want the unstaged changes", got)
		}
	})
}

func TestSetAsideTwice(t *testing.T) {
	dir := unstagedRepo(t)
	r := open(t, dir)
	if _, err := r.SetAsideUnstaged(); err != nil {
		t.Fatal(err)
	}
	gittest.WriteFile(t, dir, "modified.txt", "changed again\n")
	_, err := r.SetAsideUnstaged()
	if err == nil || !strings.Contains(err.Error(), "unstaged changes set aside earlier are still in") {
		t.Fatalf("got error %v", err)
	}
	if got := read(t, dir, "modified.txt"); got != "changed again\n" {
		t.Fatalf("the working tree changed to %q", got)
	}
}

func TestRestoreFailure(t *testing.T) {
	dir := unstagedRepo(t)
	u, err := open(t, dir).SetAsideUnstaged()
	if err != nil {
		t.Fatal(err)
	}
	// e.g. a check staged a change, the patch doesn't apply to the index anymore
	gittest.WriteFile(t, dir, "modified.txt", "staged by a check\n")
	gittest.Git(t, dir, "add", "modified.txt")
	_, err = u.Restore()
	if err == nil || !strings.Contains(err.Error(), "could not restore the unstaged changes, they are in") {
		t.Fatalf("got error %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", unstagedPatch)); err != nil {
		t.Fatal("the patch is gone although it was not restored")
	}
}
