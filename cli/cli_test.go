package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isacikgoz/gitin/git"
)

func TestMain(m *testing.M) {
	// isolate the tests from the user's git configuration
	for k, v := range map[string]string{
		"GIT_CONFIG_GLOBAL":   os.DevNull,
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_AUTHOR_NAME":     "Ada Lovelace",
		"GIT_AUTHOR_EMAIL":    "ada@example.com",
		"GIT_COMMITTER_NAME":  "Ada Lovelace",
		"GIT_COMMITTER_EMAIL": "ada@example.com",
	} {
		if err := os.Setenv(k, v); err != nil {
			panic(err)
		}
	}
	os.Exit(m.Run())
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newRepo(t *testing.T) (string, *git.Repository) {
	t.Helper()
	dir := t.TempDir()
	gitRun(t, dir, "init", "--quiet")
	writeFile(t, filepath.Join(dir, "tracked.txt"), "1\n2\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "--quiet", "-m", "root")
	r, err := git.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return dir, r
}

func statusEntry(t *testing.T, r *git.Repository, path string, staged bool) *git.StatusEntry {
	t.Helper()
	st, err := r.LoadStatus()
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range st.Entities {
		if e.String() == path && e.Indexed() == staged {
			return e
		}
	}
	t.Fatalf("no status entry for %s", path)
	return nil
}

// Hunks of new and deleted files are applied with the header git wrote,
// which includes the file mode and the /dev/null side.
func TestGenerateDiffFileKeepsHeader(t *testing.T) {
	dir, r := newRepo(t)
	writeFile(t, filepath.Join(dir, "new.txt"), "brand\nnew\n")
	if err := os.Remove(filepath.Join(dir, "tracked.txt")); err != nil {
		t.Fatal(err)
	}

	tests := map[string][]string{
		"new.txt":     {"new file mode", "--- /dev/null", "+++ b/new.txt"},
		"tracked.txt": {"deleted file mode", "--- a/tracked.txt", "+++ /dev/null"},
	}
	for path, want := range tests {
		entry := statusEntry(t, r, path, false)
		file, err := generateDiffFile(r, entry)
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
	}
}

func TestFileStatArgsSeparatePaths(t *testing.T) {
	dir, r := newRepo(t)
	gitRun(t, dir, "rm", "--quiet", "tracked.txt")
	// without "--" git can't tell a deleted path from a revision
	entry := statusEntry(t, r, "tracked.txt", true)
	out, err := r.Output(fileStatArgs(entry)...)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "deleted file mode") {
		t.Fatalf("got diff %q", out)
	}
}

func TestFileDiffArgsOfRootCommit(t *testing.T) {
	_, r := newRepo(t)
	commits, wait := r.Commits(context.Background())
	root := <-commits
	for range commits {
	}
	if err := wait(); err != nil {
		t.Fatal(err)
	}
	diff, err := root.Diff()
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.Output(fileDiffArgs(root, diff.Deltas()[0])...)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "+++ b/tracked.txt") {
		t.Fatalf("got diff %q", out)
	}
}
