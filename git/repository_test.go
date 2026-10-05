package git

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isacikgoz/gitin/internal/gittest"
)

func TestMain(m *testing.M) {
	gittest.Main(m)
}

func open(t *testing.T, dir string) *Repository {
	t.Helper()
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func resolved(t *testing.T, path string) string {
	t.Helper()
	p, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestOpen(t *testing.T) {
	t.Run("nested directory opens the working tree root", func(t *testing.T) {
		dir := gittest.NewRepo(t)
		gittest.WriteFile(t, dir, "a/b/file", "x")
		r := open(t, filepath.Join(dir, "a", "b"))
		if got, want := resolved(t, r.Path()), resolved(t, dir); got != want {
			t.Fatalf("got path %s, want %s", got, want)
		}
	})

	t.Run("nested repository opens the inner repository", func(t *testing.T) {
		outer := gittest.NewRepo(t)
		inner := filepath.Join(outer, "inner")
		gittest.Git(t, outer, "init", "--quiet", "inner")
		r := open(t, inner)
		if got, want := resolved(t, r.Path()), resolved(t, inner); got != want {
			t.Fatalf("got path %s, want %s", got, want)
		}
	})

	t.Run("bare repository opens the git directory", func(t *testing.T) {
		dir := t.TempDir()
		gittest.Git(t, dir, "init", "--quiet", "--bare", "repo.git")
		r := open(t, filepath.Join(dir, "repo.git"))
		if got, want := resolved(t, r.Path()), resolved(t, filepath.Join(dir, "repo.git")); got != want {
			t.Fatalf("got path %s, want %s", got, want)
		}
	})

	t.Run("inside the git directory opens the git directory", func(t *testing.T) {
		dir := gittest.NewRepo(t)
		r := open(t, filepath.Join(dir, ".git", "refs"))
		if got, want := resolved(t, r.Path()), resolved(t, filepath.Join(dir, ".git")); got != want {
			t.Fatalf("got path %s, want %s", got, want)
		}
	})

	t.Run("not a repository", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
		_, err := Open(dir)
		if !errors.Is(err, ErrCannotOpenRepo) {
			t.Fatalf("got error %v, want %v", err, ErrCannotOpenRepo)
		}
		if !strings.Contains(err.Error(), "not a git repository") {
			t.Fatalf("error %q does not include git's message", err)
		}
	})
}

func TestOutputReturnsGitsMessage(t *testing.T) {
	r := open(t, gittest.NewRepo(t))
	out, err := r.Output("rev-parse", "--verify", "no-such-branch")
	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("got error %T %v, want *CommandError", err, err)
	}
	if got, want := err.Error(), "Needed a single revision"; got != want {
		t.Fatalf("got message %q, want %q", got, want)
	}
	if strings.Join(cmdErr.Args, " ") != "rev-parse --verify no-such-branch" {
		t.Fatalf("got args %q", cmdErr.Args)
	}
	if ExitCode(err) != 128 {
		t.Fatalf("got exit code %d, want 128", ExitCode(err))
	}
	if len(out) != 0 {
		t.Fatalf("got output %q", out)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatal("CommandError does not unwrap to the exit error")
	}
}

func TestOutputKeepsStdoutOfFailures(t *testing.T) {
	dir := gittest.NewRepo(t)
	gittest.WriteFile(t, dir, "new.txt", "x\n")
	out, err := open(t, dir).Output("diff", "--no-index", "--", "/dev/null", "new.txt")
	if ExitCode(err) != 1 {
		t.Fatalf("got error %v, want exit code 1", err)
	}
	if !strings.Contains(string(out), "+x") {
		t.Fatalf("got output %q", out)
	}
}

func TestCommandErrorWithoutMessage(t *testing.T) {
	err := &CommandError{Args: []string{"status"}, Err: errors.New("exit status 2")}
	if got, want := err.Error(), "git status: exit status 2"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if ExitCode(errors.New("other")) != -1 {
		t.Fatal("ExitCode of a non exit error is not -1")
	}
}

func TestRunUsesGivenOutputs(t *testing.T) {
	r := open(t, gittest.NewRepo(t))
	var stdout strings.Builder
	cmd := r.Command("rev-parse", "--is-inside-work-tree")
	cmd.Stdout = &stdout
	out, err := Run(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 || stdout.String() != "true\n" {
		t.Fatalf("got returned %q and written %q", out, stdout.String())
	}
}

func TestMerging(t *testing.T) {
	dir := gittest.NewRepo(t)
	gittest.Commit(t, dir, "base", map[string]string{"a": "1"})
	gittest.Git(t, dir, "checkout", "--quiet", "-b", "topic")
	gittest.Commit(t, dir, "topic", map[string]string{"a": "topic"})
	gittest.Git(t, dir, "checkout", "--quiet", "main")
	gittest.Commit(t, dir, "main", map[string]string{"a": "main"})
	r := open(t, dir)
	if r.Merging() {
		t.Fatal("merging before a merge")
	}
	cmd := exec.Command("git", "merge", "--quiet", "topic")
	cmd.Dir = dir
	_ = cmd.Run() // conflicts
	if !r.Merging() {
		t.Fatal("not merging during a merge")
	}
}
