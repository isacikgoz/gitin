package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isacikgoz/gitin/git"
	"github.com/isacikgoz/gitin/internal/gittest"
	"github.com/waigani/diffparser"
)

// statusRepo returns a repository with a staged, an unstaged, a deleted and
// an untracked file
func statusRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.NewRepo(t)
	gittest.Commit(t, dir, "base", map[string]string{"staged.txt": "1\n", "unstaged.txt": "1\n", "deleted.txt": "1\n"})
	gittest.WriteFile(t, dir, "staged.txt", "2\n")
	gittest.Git(t, dir, "add", "staged.txt")
	gittest.WriteFile(t, dir, "unstaged.txt", "2\n")
	if err := os.Remove(filepath.Join(dir, "deleted.txt")); err != nil {
		t.Fatal(err)
	}
	gittest.WriteFile(t, dir, "untracked.txt", "new\n")
	return dir
}

func newTestStatus(t *testing.T, dir string) *status {
	t.Helper()
	s, err := newStatus(open(t, dir), opts)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// entry returns the status entry of path shown by the prompt
func entry(t *testing.T, s *status, path string, staged bool) *git.StatusEntry {
	t.Helper()
	items, _ := s.prompt.State().List.Items()
	for _, item := range items {
		if e := item.(*git.StatusEntry); e.String() == path && e.Indexed() == staged {
			return e
		}
	}
	t.Fatalf("%s (staged %v) is not listed in %v", path, staged, items)
	return nil
}

func TestStatusPromptOfCleanWorkingTree(t *testing.T) {
	out := captureTerminal(t)
	dir := gittest.NewRepo(t)
	gittest.Commit(t, dir, "base", map[string]string{"a": "1"})
	p, err := StatusPrompt(open(t, dir), opts)
	if err != nil || p != nil {
		t.Fatalf("got prompt %v, error %v", p, err)
	}
	if !strings.Contains(out.String(), "Nothing to commit, working tree clean") {
		t.Fatalf("got %q", out.String())
	}
}

func TestStatusPromptOutsideWorkingTree(t *testing.T) {
	dir := gittest.NewRepo(t)
	if _, err := StatusPrompt(open(t, filepath.Join(dir, ".git")), opts); err == nil ||
		!strings.Contains(err.Error(), "work tree") {
		t.Fatalf("got error %v", err)
	}
}

func TestStatusEntries(t *testing.T) {
	dir := statusRepo(t)
	p, err := StatusPrompt(open(t, dir), opts)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(visible(p), " "); got != "deleted.txt staged.txt unstaged.txt untracked.txt" {
		t.Fatalf("got %s", got)
	}
}

func TestStatusAddAndReset(t *testing.T) {
	dir := statusRepo(t)
	s := newTestStatus(t, dir)

	if err := s.addResetEntry(entry(t, s, "untracked.txt", false)); err != nil {
		t.Fatal(err)
	}
	if err := s.addResetEntry(entry(t, s, "deleted.txt", false)); err != nil {
		t.Fatal(err)
	}
	if err := s.addResetEntry(entry(t, s, "staged.txt", true)); err != nil {
		t.Fatal(err)
	}
	want := "D  deleted.txt\n M staged.txt\n M unstaged.txt\nA  untracked.txt"
	if got := shortStatus(t, dir); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	if got := strings.Join(visible(s.prompt), " "); got != "deleted.txt staged.txt unstaged.txt untracked.txt" {
		t.Fatalf("list was not reloaded: %s", got)
	}

	if err := s.addAllEntries(nil); err != nil {
		t.Fatal(err)
	}
	if got := shortStatus(t, dir); got != "D  deleted.txt\nM  staged.txt\nM  unstaged.txt\nA  untracked.txt" {
		t.Fatalf("after add all: got\n%s", got)
	}
	if err := s.resetAllEntries(nil); err != nil {
		t.Fatal(err)
	}
	if got := shortStatus(t, dir); got != " D deleted.txt\n M staged.txt\n M unstaged.txt\n?? untracked.txt" {
		t.Fatalf("after reset all: got\n%s", got)
	}
}

func TestStatusResetBeforeFirstCommit(t *testing.T) {
	dir := gittest.NewRepo(t)
	gittest.WriteFile(t, dir, "a", "1")
	gittest.Git(t, dir, "add", "a")
	s := newTestStatus(t, dir)
	if err := s.addResetEntry(entry(t, s, "a", true)); err != nil {
		t.Fatal(err)
	}
	if got := shortStatus(t, dir); got != "?? a" {
		t.Fatalf("got %q", got)
	}
}

func TestStatusDiscard(t *testing.T) {
	dir := statusRepo(t)
	s := newTestStatus(t, dir)

	err := s.discardEntry(entry(t, s, "staged.txt", true))
	if err == nil || !strings.Contains(err.Error(), "unstage") {
		t.Fatalf("discarding a staged entry: got error %v", err)
	}
	for _, path := range []string{"unstaged.txt", "deleted.txt", "untracked.txt"} {
		if err := s.discardEntry(entry(t, s, path, false)); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	if got := shortStatus(t, dir); got != "M  staged.txt" {
		t.Fatalf("got %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "untracked.txt")); !os.IsNotExist(err) {
		t.Fatal("untracked file was not removed")
	}
}

func TestStatusDiscardUntrackedDirectory(t *testing.T) {
	dir := gittest.NewRepo(t)
	gittest.Commit(t, dir, "base", map[string]string{"a": "1"})
	gittest.WriteFile(t, dir, "new/dir/file", "x")
	gittest.WriteFile(t, dir, "a", "2")
	s := newTestStatus(t, dir)
	if err := s.discardEntry(entry(t, s, "new/", false)); err != nil {
		t.Fatal(err)
	}
	if got := shortStatus(t, dir); got != " M a" {
		t.Fatalf("got %q", got)
	}
}

func TestStatusShowsGitErrors(t *testing.T) {
	dir := statusRepo(t)
	s := newTestStatus(t, dir)
	e := entry(t, s, "unstaged.txt", false)
	gittest.WriteFile(t, dir, ".git/index.lock", "")
	err := s.addResetEntry(e)
	if err == nil || !strings.Contains(err.Error(), "index.lock") {
		t.Fatalf("got error %v", err)
	}
}

func TestStatusDiffs(t *testing.T) {
	dir := statusRepo(t)
	gittest.Git(t, dir, "rm", "--quiet", "--cached", "unstaged.txt")
	s := newTestStatus(t, dir)

	tests := []struct {
		path   string
		staged bool
		want   string
	}{
		{"staged.txt", true, "+2"},
		{"deleted.txt", false, "deleted file mode"},
		{"unstaged.txt", true, "deleted file mode"},
		{"unstaged.txt", false, "+2"},
	}
	for _, tt := range tests {
		out := captureTerminal(t)
		if err := s.onSelect(entry(t, s, tt.path, tt.staged)); err != nil {
			t.Fatalf("%s: %v", tt.path, err)
		}
		if !strings.Contains(out.String(), tt.want) {
			t.Errorf("%s (staged %v): got %q, want %q", tt.path, tt.staged, out.String(), tt.want)
		}
	}

	out := captureTerminal(t)
	// "git diff --no-index" exits with 1 because the files differ
	if err := s.onSelect(entry(t, s, "untracked.txt", false)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "+new") {
		t.Fatalf("got %q", out.String())
	}
}

func TestStatusInfo(t *testing.T) {
	s := newTestStatus(t, statusRepo(t))
	if got := lines(s.info(nil)); got[0] != "On branch main" {
		t.Fatalf("got %q", got)
	}
}

func TestStatusCommit(t *testing.T) {
	dir := statusRepo(t)
	s := newTestStatus(t, dir)
	out := captureTerminal(t)
	setEditor(t, "commit from gitin")

	if err := s.commit(nil); err != nil {
		t.Fatal(err)
	}
	if got := gittest.Git(t, dir, "log", "-1", "--format=%s"); got != "commit from gitin" {
		t.Fatalf("got last commit %q", got)
	}
	if !strings.Contains(out.String(), "staged.txt | 2 +-") {
		t.Fatalf("commit stat was not shown: %q", out.String())
	}
	if got := strings.Join(visible(s.prompt), " "); got != "deleted.txt unstaged.txt untracked.txt" {
		t.Fatalf("list was not reloaded: %s", got)
	}

	setEditor(t, "amended")
	if err := s.amend(nil); err != nil {
		t.Fatal(err)
	}
	if got := gittest.Git(t, dir, "log", "-2", "--format=%s"); got != "amended\nbase" {
		t.Fatalf("got last commits %q", got)
	}
}

func TestStatusCommitFailures(t *testing.T) {
	dir := statusRepo(t)
	s := newTestStatus(t, dir)
	captureTerminal(t)

	setEditor(t, "")
	err := s.commit(nil)
	if err == nil || !strings.Contains(err.Error(), "empty commit message") {
		t.Fatalf("empty message: got error %v", err)
	}

	gittest.WriteFile(t, dir, ".git/hooks/pre-commit", "#!/bin/sh\necho 'lint failed: fix it' >&2\nexit 1\n")
	if err := os.Chmod(filepath.Join(dir, ".git/hooks/pre-commit"), 0o755); err != nil {
		t.Fatal(err)
	}
	setEditor(t, "rejected")
	err = s.commit(nil)
	if err == nil || !strings.Contains(err.Error(), "lint failed: fix it") {
		t.Fatalf("rejecting hook: got error %v", err)
	}
	if got := gittest.Git(t, dir, "log", "-1", "--format=%s"); got != "base" {
		t.Fatalf("got last commit %q", got)
	}
}

func TestStatusCommitLastChange(t *testing.T) {
	dir := gittest.NewRepo(t)
	gittest.Commit(t, dir, "base", map[string]string{"a": "1"})
	gittest.WriteFile(t, dir, "a", "2")
	gittest.Git(t, dir, "add", "a")
	s := newTestStatus(t, dir)
	captureTerminal(t)
	setEditor(t, "last change")
	// the working tree is clean afterwards, the prompt quits
	if err := s.commit(nil); err != nil {
		t.Fatal(err)
	}
	if got := shortStatus(t, dir); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestGenerateDiffFileKeepsHeader(t *testing.T) {
	dir := statusRepo(t)
	s := newTestStatus(t, dir)
	r := open(t, dir)

	// hunks of new and deleted files are applied with the header git
	// wrote, it has the file mode and the /dev/null side
	tests := map[string][]string{
		"untracked.txt": {"new file mode", "--- /dev/null", "+++ b/untracked.txt"},
		"deleted.txt":   {"deleted file mode", "--- a/deleted.txt", "+++ /dev/null"},
		"unstaged.txt":  {"--- a/unstaged.txt", "+++ b/unstaged.txt"},
	}
	for path, want := range tests {
		e := entry(t, s, path, false)
		file, err := generateDiffFile(r, e)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for _, line := range want {
			if !strings.Contains(file.DiffHeader, line) {
				t.Errorf("%s: header %q does not contain %q", path, file.DiffHeader, line)
			}
		}
		if len(file.Hunks) != 1 {
			t.Fatalf("%s: got %d hunks, want 1", path, len(file.Hunks))
		}
		if err := applyPatchCmd(r, e, hunkPatch(file)); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	want := "D  deleted.txt\nM  staged.txt\nM  unstaged.txt\nA  untracked.txt"
	if got := shortStatus(t, dir); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}

	// and unstage one again
	s = newTestStatus(t, dir)
	e := entry(t, s, "untracked.txt", true)
	file, err := generateDiffFile(r, e)
	if err != nil {
		t.Fatal(err)
	}
	if err := applyPatchCmd(r, e, hunkPatch(file)); err != nil {
		t.Fatal(err)
	}
	if got := shortStatus(t, dir); !strings.Contains(got, "?? untracked.txt") {
		t.Fatalf("got\n%s", got)
	}
}

// hunkPatch builds the patch of the first hunk like the hunk editor does
func hunkPatch(file *diffparser.DiffFile) string {
	hunk := file.Hunks[0]
	patch := fmt.Sprintf("%s\n@@ -%d,%d +%d,%d @@ %s", file.DiffHeader,
		hunk.OrigRange.Start, hunk.OrigRange.Length, hunk.NewRange.Start, hunk.NewRange.Length, hunk.HunkHeader)
	for _, line := range hunk.WholeRange.Lines {
		prefix := " "
		switch line.Mode {
		case diffparser.ADDED:
			prefix = "+"
		case diffparser.REMOVED:
			prefix = "-"
		}
		patch += "\n" + prefix + line.Content
	}
	return patch
}

func TestGenerateDiffFileWithUserDiffSettings(t *testing.T) {
	dir := statusRepo(t)
	for _, setting := range [][]string{
		{"color.diff", "always"},
		{"diff.noprefix", "true"},
		{"diff.external", "false"},
	} {
		gittest.Git(t, dir, "config", setting[0], setting[1])
	}
	s := newTestStatus(t, dir)
	e := entry(t, s, "unstaged.txt", false)
	file, err := generateDiffFile(open(t, dir), e)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(file.DiffHeader, "--- a/unstaged.txt") || strings.Contains(file.DiffHeader, "\x1b[") {
		t.Fatalf("got header %q", file.DiffHeader)
	}
}

func TestGenerateDiffFileWithoutChanges(t *testing.T) {
	dir := gittest.NewRepo(t)
	gittest.Commit(t, dir, "base", map[string]string{"a": "1"})
	if err := os.Chmod(filepath.Join(dir, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := newTestStatus(t, dir)
	_, err := generateDiffFile(open(t, dir), entry(t, s, "a", false))
	if err == nil || !strings.Contains(err.Error(), "no changes to stage by hunk") {
		t.Fatalf("got error %v", err)
	}
}

func TestFileStatArgs(t *testing.T) {
	dir := statusRepo(t)
	s := newTestStatus(t, dir)
	tests := []struct {
		path   string
		staged bool
		want   string
	}{
		{"staged.txt", true, "diff --cached -- staged.txt"},
		{"unstaged.txt", false, "diff -- unstaged.txt"},
		{"untracked.txt", false, "diff --no-index -- /dev/null untracked.txt"},
	}
	for _, tt := range tests {
		if got := strings.Join(fileStatArgs(entry(t, s, tt.path, tt.staged)), " "); got != tt.want {
			t.Errorf("got %q, want %q", got, tt.want)
		}
	}
}

func TestStatusCommitNeedsStagedChanges(t *testing.T) {
	dir := gittest.NewRepo(t)
	gittest.Commit(t, dir, "base", map[string]string{"a": "1"})
	gittest.WriteFile(t, dir, "a", "2")
	s := newTestStatus(t, dir)
	captureTerminal(t)
	err := s.commit(nil)
	if err == nil || err.Error() != "nothing to commit, stage changes with space or a first" {
		t.Fatalf("got error %v", err)
	}
}

// A merge is committed although nothing is staged, e.g. when its result is
// the version of the current branch
func TestStatusCommitMergeWithoutStagedChanges(t *testing.T) {
	dir := gittest.NewRepo(t)
	gittest.Commit(t, dir, "base", map[string]string{"a": "1", "b": "1"})
	gittest.Git(t, dir, "checkout", "--quiet", "-b", "topic")
	gittest.Commit(t, dir, "topic", map[string]string{"a": "topic"})
	gittest.Git(t, dir, "checkout", "--quiet", "main")
	gittest.Commit(t, dir, "main", map[string]string{"a": "main"})
	cmd := exec.Command("git", "merge", "--quiet", "topic")
	cmd.Dir = dir
	_ = cmd.Run() // conflicts
	gittest.Git(t, dir, "checkout", "--ours", "a")
	gittest.Git(t, dir, "add", "a")
	gittest.WriteFile(t, dir, "b", "unstaged")

	s := newTestStatus(t, dir)
	captureTerminal(t)
	setEditor(t, "merge topic")
	if err := s.commit(nil); err != nil {
		t.Fatal(err)
	}
	if got := gittest.Git(t, dir, "log", "-1", "--format=%s %p"); !strings.HasPrefix(got, "merge topic ") || len(strings.Fields(got)) != 4 {
		t.Fatalf("got last commit %q, want a merge commit", got)
	}
}
