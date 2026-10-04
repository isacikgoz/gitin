package e2e

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isacikgoz/gitin/internal/gittest"
)

func TestOutsideRepository(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	for _, mode := range []string{"log", "status", "branch"} {
		out, code := runGitin(t, dir, mode)
		if code != 1 || !strings.Contains(out, "cannot load repository: not a git repository") {
			t.Errorf("%s: got exit code %d and %q", mode, code, out)
		}
	}
}

func TestVersionAndHelp(t *testing.T) {
	out, code := runGitin(t, t.TempDir(), "--version")
	if code != 0 || !strings.HasPrefix(out, "gitin version ") {
		t.Fatalf("got exit code %d and %q", code, out)
	}
	out, code = runGitin(t, t.TempDir(), "--help")
	for _, want := range []string{"log", "status", "branch", "GITIN_LINESIZE", "GITIN_STARTINSEARCH", "GITIN_DISABLECOLOR", "GITIN_VIMKEYS"} {
		if code != 0 || !strings.Contains(out, want) {
			t.Errorf("help does not mention %s (exit code %d):\n%s", want, code, out)
		}
	}
	if strings.Contains(out, "hunk-editor") {
		t.Errorf("help shows the internal hunk editor command:\n%s", out)
	}
}

func TestOptionsFromEnvironment(t *testing.T) {
	dir := gittest.NewRepo(t)
	for _, msg := range []string{"one", "two", "three", "four"} {
		gittest.Commit(t, dir, msg, nil)
	}

	t.Run("line size", func(t *testing.T) {
		s := start(t, dir, []string{"log"}, "GITIN_LINESIZE=2")
		frame := s.waitFrame("Commits", 0, "four", "three")
		if strings.Contains(frame, "two") {
			t.Fatalf("more than 2 lines:\n%s", frame)
		}
	})

	t.Run("start in search", func(t *testing.T) {
		s := start(t, dir, []string{"log"}, "GITIN_STARTINSEARCH=true")
		s.send("t", "w")
		s.waitFrame("Search Commits tw", 0, "two")
	})

	t.Run("vim keys", func(t *testing.T) {
		s := start(t, dir, []string{"log"}, "GITIN_VIMKEYS=false")
		s.waitFrame("Commits", 0, "four")
		s.send("j")
		m := s.mark()
		s.send(down) // j did not move the cursor, so down selects the second commit
		s.waitSelected("Commits", m, "three")
	})

	t.Run("colors", func(t *testing.T) {
		s := start(t, dir, []string{"log"})
		s.waitFrame("Commits", 0, "four")
		if !strings.Contains(s.raw(), "\x1b[36m") {
			t.Fatal("no colors")
		}
		s = start(t, dir, []string{"log"}, "GITIN_DISABLECOLOR=true")
		s.waitFrame("Commits", 0, "four")
		if strings.Contains(s.raw(), "\x1b[36m") {
			t.Fatal("colors although they are disabled")
		}
	})
}

func TestRepositoryFormats(t *testing.T) {
	// these repositories need newer git versions, skip the ones it can't create
	for _, tt := range []struct {
		name string
		args []string
	}{
		{"sha256", []string{"init", "--quiet", "--object-format=sha256"}},
		{"reftable", []string{"init", "--quiet", "--ref-format=reftable"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if _, err := gitOutput(dir, tt.args...); err != nil {
				t.Skipf("git can't create the repository: %v", err)
			}
			gittest.Git(t, dir, "symbolic-ref", "HEAD", "refs/heads/main")
			gittest.Commit(t, dir, tt.name+" commit", map[string]string{"a": "1"})
			s := start(t, dir, []string{"log"})
			s.waitFrame("Commits", 0, tt.name+" commit")
		})
	}

	t.Run("partial clone", func(t *testing.T) {
		dir := gittest.NewRepo(t)
		gittest.Commit(t, dir, "first", map[string]string{"a": "1"})
		gittest.Commit(t, dir, "second", map[string]string{"a": "2"})
		gittest.Git(t, dir, "config", "uploadpack.allowFilter", "true")
		clone := filepath.Join(t.TempDir(), "clone")
		if out, err := gitOutput(dir, "clone", "--quiet", "--filter=blob:none", "file://"+dir, clone); err != nil {
			t.Skipf("git can't create a partial clone: %v\n%s", err, out)
		}
		s := start(t, clone, []string{"log"})
		s.waitFrame("Commits", 0, "second", "first")
		// the blobs of older commits are fetched when they are needed
		s.send(down, enter)
		s.waitFrame("Files", 0, "[A] a")
	})
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}
