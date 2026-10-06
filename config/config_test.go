package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// repo returns the working tree and git directory of a repository
func repo(t *testing.T) (worktree, gitDir string) {
	t.Helper()
	worktree = t.TempDir()
	return worktree, filepath.Join(worktree, ".git")
}

func names(checks []Check) string {
	var out []string
	for _, c := range checks {
		out = append(out, c.Name)
	}
	return strings.Join(out, ",")
}

func TestLoadShared(t *testing.T) {
	worktree, gitDir := repo(t)
	write(t, filepath.Join(worktree, ".gitin.yml"), `
commit:
  checks:
    - name: Format
      run: gofmt -l .
# checks run before pushing
push:
  checks:
    - name: Lint
      run: golangci-lint run
    - run: go test ./...
`)
	c, err := Load(worktree, gitDir)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(c.Files) != "[.gitin.yml]" || c.Personal != filepath.Join(".git", "gitin", "config.yml") {
		t.Fatalf("got files %v personal %q", c.Files, c.Personal)
	}
	if got := c.Commit.Checks[0]; got.Name != "Format" || got.Run != "gofmt -l ." {
		t.Fatalf("got commit check %+v", got)
	}
	if got := c.Push.Checks[0]; got.Name != "Lint" || got.Run != "golangci-lint run" {
		t.Fatalf("got first push check %+v", got)
	}
	// the name defaults to the command
	if got := c.Push.Checks[1]; got.Name != "go test ./..." || got.Run != "go test ./..." {
		t.Fatalf("got second push check %+v", got)
	}
}

func TestLoadPersonal(t *testing.T) {
	worktree, gitDir := repo(t)
	write(t, filepath.Join(gitDir, "gitin", "config.yml"), "push:\n  checks:\n    - name: Mine\n      run: make check\n")
	c, err := Load(worktree, gitDir)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(c.Files) != "["+filepath.Join(".git", "gitin", "config.yml")+"]" || names(c.Push.Checks) != "Mine" {
		t.Fatalf("got files %v and checks %s", c.Files, names(c.Push.Checks))
	}
}

func TestLoadBoth(t *testing.T) {
	worktree, gitDir := repo(t)
	write(t, filepath.Join(worktree, ".gitin.yml"), "commit:\n  checks:\n    - name: Team format\n      run: x\npush:\n  checks:\n    - name: Team tests\n      run: x\n")
	write(t, filepath.Join(gitDir, "gitin", "config.yml"), "commit:\n  checks:\n    - name: My format\n      run: x\npush:\n  checks:\n    - name: My secrets\n      run: x\n")
	c, err := Load(worktree, gitDir)
	if err != nil {
		t.Fatal(err)
	}
	// the team's checks run first
	if names(c.Commit.Checks) != "Team format,My format" || names(c.Push.Checks) != "Team tests,My secrets" {
		t.Fatalf("got commit checks %s and push checks %s", names(c.Commit.Checks), names(c.Push.Checks))
	}
	if fmt.Sprint(c.Files) != "[.gitin.yml "+filepath.Join(".git", "gitin", "config.yml")+"]" {
		t.Fatalf("got files %v", c.Files)
	}
}

func TestLoadWithoutFiles(t *testing.T) {
	c, err := Load(repo(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Files) != 0 || len(c.Push.Checks) != 0 || len(c.Commit.Checks) != 0 {
		t.Fatalf("got %+v", c)
	}
}

// a linked worktree has its own working tree, the personal file is in the
// git directory of the main one
func TestLoadOutsideTheWorkingTree(t *testing.T) {
	worktree, commonDir := t.TempDir(), filepath.Join(t.TempDir(), ".git")
	personal := filepath.Join(commonDir, "gitin", "config.yml")
	write(t, personal, "push:\n  checks:\n    - run: make\n")
	c, err := Load(worktree, commonDir)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(c.Files) != "["+personal+"]" || c.Personal != personal {
		t.Fatalf("got files %v personal %q, want the absolute path", c.Files, c.Personal)
	}

	// a bare repository has no working tree
	if c, err = Load("", commonDir); err != nil || len(c.Push.Checks) != 1 {
		t.Fatalf("got %+v, %v", c, err)
	}
}

func TestLoadOtherExtension(t *testing.T) {
	worktree, gitDir := repo(t)
	write(t, filepath.Join(worktree, ".gitin.yaml"), "push:\n  checks:\n    - run: make test\n")
	c, err := Load(worktree, gitDir)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(c.Files) != "[.gitin.yaml]" || len(c.Push.Checks) != 1 {
		t.Fatalf("got %+v", c)
	}

	write(t, filepath.Join(worktree, ".gitin.yml"), "")
	if _, err := Load(worktree, gitDir); err == nil || !strings.Contains(err.Error(), "both .gitin.yml and .gitin.yaml exist") {
		t.Fatalf("got error %v", err)
	}
}

func TestLoadErrors(t *testing.T) {
	tests := map[string]struct {
		content string
		want    string
	}{
		"empty file":        {"", ""},
		"only comments":     {"# nothing yet\n", ""},
		"no checks":         {"push:\n", ""},
		"typo in a key":     {"push:\n  check:\n    - run: x\n", "line 2: field check not found"},
		"unknown check key": {"push:\n  checks:\n    - run: x\n      timeout: 5m\n", "line 4: field timeout not found"},
		"missing command":   {"push:\n  checks:\n    - run: x\n    - name: Lint\n", "push.checks[1]: run is missing"},
		"missing in commit": {"commit:\n  checks:\n    - name: Format\n", "commit.checks[0]: run is missing"},
		"not a list":        {"push:\n  checks: make test\n", "line 2"},
		"invalid yaml":      {"push: [\n", "yaml:"},
	}
	for name, tt := range tests {
		for _, file := range []string{".gitin.yml", filepath.Join(".git", "gitin", "config.yml")} {
			t.Run(name+" in "+file, func(t *testing.T) {
				worktree, gitDir := repo(t)
				write(t, filepath.Join(worktree, file), tt.content)
				_, err := Load(worktree, gitDir)
				if tt.want == "" {
					if err != nil {
						t.Fatal(err)
					}
					return
				}
				// errors name the file they are in
				if err == nil || !strings.Contains(err.Error(), tt.want) || !strings.HasPrefix(err.Error(), file+": ") {
					t.Fatalf("got error %v, want one of %s with %q", err, file, tt.want)
				}
			})
		}
	}
}

func TestLoadUnreadableFile(t *testing.T) {
	worktree, gitDir := repo(t)
	if err := os.MkdirAll(filepath.Join(gitDir, "gitin", "config.yml"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(worktree, gitDir); err == nil {
		t.Fatal("no error for a directory named config.yml")
	}
}

// The example of the README must stay valid
func TestREADMEExample(t *testing.T) {
	readme, err := os.ReadFile("../README.md")
	if err != nil {
		t.Fatal(err)
	}
	_, rest, found := strings.Cut(string(readme), "```yaml\n")
	example, _, closed := strings.Cut(rest, "```")
	if !found || !closed {
		t.Fatal("the README has no yaml example")
	}
	worktree, gitDir := repo(t)
	write(t, filepath.Join(gitDir, "gitin", "config.yml"), example)
	c, err := Load(worktree, gitDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Push.Checks) == 0 || len(c.Commit.Checks) == 0 {
		t.Fatal("the example has no commit and push checks")
	}
}
