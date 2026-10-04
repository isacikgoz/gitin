package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/isacikgoz/gitin/git"
	"github.com/isacikgoz/gitin/internal/gittest"
	"github.com/isacikgoz/gitin/prompt"
	"github.com/isacikgoz/gitin/term"
)

func TestMain(m *testing.M) {
	gittest.Main(m)
}

var opts = &prompt.Options{LineSize: 5}

func open(t *testing.T, dir string) *git.Repository {
	t.Helper()
	r, err := git.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// captureTerminal collects the output of interactive git commands, pagers
// print to it directly
func captureTerminal(t *testing.T) *bytes.Buffer {
	t.Helper()
	var out bytes.Buffer
	oldIn, oldOut := stdin, stdout
	stdin, stdout = strings.NewReader(""), &out
	t.Cleanup(func() { stdin, stdout = oldIn, oldOut })
	t.Setenv("GIT_PAGER", "cat")
	return &out
}

// setEditor makes git use an editor that writes message
func setEditor(t *testing.T, message string) {
	t.Helper()
	script := filepath.Join(t.TempDir(), "editor")
	content := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' %q > \"$1\"\n", message)
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_EDITOR", script)
}

func commits(t *testing.T, r *git.Repository) []*git.Commit {
	t.Helper()
	ch, wait := r.Commits(context.Background())
	var all []*git.Commit
	for c := range ch {
		all = append(all, c)
	}
	if err := wait(); err != nil {
		t.Fatal(err)
	}
	return all
}

// text returns the characters of cells
func text(cells []term.Cell) string {
	var s strings.Builder
	for _, c := range cells {
		s.WriteRune(c.Ch)
	}
	return s.String()
}

// lines returns the text of every line of a grid
func lines(grid [][]term.Cell) []string {
	var out []string
	for _, line := range grid {
		out = append(out, text(line))
	}
	return out
}

func hasAttr(c term.Cell, attr color.Attribute) bool {
	for _, a := range c.Attr {
		if a == attr {
			return true
		}
	}
	return false
}

// visible returns the texts of the visible items of a prompt
func visible(p *prompt.Prompt) []string {
	items, _ := p.State().List.Items()
	var out []string
	for _, item := range items {
		out = append(out, fmt.Sprint(item))
	}
	return out
}

// selected returns the item under the cursor of a prompt
func selected(t *testing.T, p *prompt.Prompt) interface{} {
	t.Helper()
	items, active := p.State().List.Items()
	if active == prompt.NotFound {
		t.Fatal("no item is selected")
	}
	return items[active]
}

// shortStatus returns "git status --short" of dir
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
