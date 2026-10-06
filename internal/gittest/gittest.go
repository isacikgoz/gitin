// Package gittest creates git repositories for tests. It only uses git
// features available in the oldest git version gitin supports.
package gittest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Isolate makes git ignore the configuration of the user running the tests
// and sets a fixed author. Call it from TestMain, it changes HOME.
func Isolate() error {
	home, err := os.MkdirTemp("", "gittest-home")
	if err != nil {
		return err
	}
	for k, v := range map[string]string{
		"HOME":                home,
		"XDG_CONFIG_HOME":     filepath.Join(home, ".config"),
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_CONFIG_GLOBAL":   os.DevNull,
		"GIT_TERMINAL_PROMPT": "0",
		"GIT_AUTHOR_NAME":     "Ada Lovelace",
		"GIT_AUTHOR_EMAIL":    "ada@example.com",
		"GIT_COMMITTER_NAME":  "Ada Lovelace",
		"GIT_COMMITTER_EMAIL": "ada@example.com",
	} {
		if err := os.Setenv(k, v); err != nil {
			return err
		}
	}
	return nil
}

// Main isolates git and runs the tests, use it as TestMain.
func Main(m *testing.M) {
	if err := Isolate(); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// Git runs git in dir and returns its trimmed output
func Git(t testing.TB, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// WriteFile writes content to dir/name, creating directories as needed
func WriteFile(t testing.TB, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// NewRepo creates a repository on branch main without commits
func NewRepo(t testing.TB) string {
	t.Helper()
	dir := t.TempDir()
	Git(t, dir, "init", "--quiet")
	Git(t, dir, "symbolic-ref", "HEAD", "refs/heads/main")
	return dir
}

// Commit writes the files, commits all changes and returns the commit hash
func Commit(t testing.TB, dir, message string, files map[string]string) string {
	t.Helper()
	for name, content := range files {
		WriteFile(t, dir, name, content)
	}
	Git(t, dir, "add", "--all")
	Git(t, dir, "commit", "--quiet", "--allow-empty", "-m", message)
	return Git(t, dir, "rev-parse", "HEAD")
}

// Clone clones the repository in dir and returns the path of the clone
func Clone(t testing.TB, dir string, args ...string) string {
	t.Helper()
	clone := filepath.Join(t.TempDir(), "clone")
	Git(t, dir, append(append([]string{"clone", "--quiet"}, args...), "file://"+dir, clone)...)
	return clone
}

// NewRemote creates a bare repository with a commit on main, the remote,
// and a clone of it on main that tracks origin/main
func NewRemote(t testing.TB) (remote, clone string) {
	t.Helper()
	work := NewRepo(t)
	Commit(t, work, "initial commit", map[string]string{"README": "hello\n"})
	remote = filepath.Join(t.TempDir(), "remote.git")
	Git(t, work, "clone", "--quiet", "--bare", work, remote)
	return remote, Clone(t, remote)
}
