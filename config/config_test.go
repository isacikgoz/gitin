package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, ".gitin.yml", `
# checks run before pushing
push:
  checks:
    - name: Lint
      run: golangci-lint run
    - run: go test ./...
`)
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.File != ".gitin.yml" || len(c.Push.Checks) != 2 {
		t.Fatalf("got %+v", c)
	}
	if got := c.Push.Checks[0]; got.Name != "Lint" || got.Run != "golangci-lint run" {
		t.Fatalf("got first check %+v", got)
	}
	// the name defaults to the command
	if got := c.Push.Checks[1]; got.Name != "go test ./..." || got.Run != "go test ./..." {
		t.Fatalf("got second check %+v", got)
	}
}

func TestLoadWithoutFile(t *testing.T) {
	c, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if c.File != "" || len(c.Push.Checks) != 0 {
		t.Fatalf("got %+v", c)
	}
}

func TestLoadOtherExtension(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, ".gitin.yaml", "push:\n  checks:\n    - run: make test\n")
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.File != ".gitin.yaml" || len(c.Push.Checks) != 1 {
		t.Fatalf("got %+v", c)
	}

	write(t, dir, ".gitin.yml", "")
	if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), "both .gitin.yml and .gitin.yaml exist") {
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
		"not a list":        {"push:\n  checks: make test\n", "line 2"},
		"invalid yaml":      {"push: [\n", "yaml:"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			write(t, dir, ".gitin.yml", tt.content)
			_, err := Load(dir)
			if tt.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) || !strings.HasPrefix(err.Error(), ".gitin.yml: ") {
				t.Fatalf("got error %v, want one with %q", err, tt.want)
			}
		})
	}
}

func TestLoadUnreadableFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".gitin.yml"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("no error for a directory named .gitin.yml")
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
	dir := t.TempDir()
	write(t, dir, ".gitin.yml", example)
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Push.Checks) == 0 {
		t.Fatal("the example has no checks")
	}
}
