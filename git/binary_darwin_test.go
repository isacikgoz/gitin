package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// developerDir creates a developer directory with a git at
// <root>/<sub>/usr/bin/git
func developerDir(t *testing.T, sub string, mode os.FileMode) (root, git string) {
	t.Helper()
	root = t.TempDir()
	git = filepath.Join(root, sub, "usr", "bin", "git")
	if err := os.MkdirAll(filepath.Dir(git), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(git, []byte("#!/bin/sh\n"), mode); err != nil {
		t.Fatal(err)
	}
	return root, git
}

func found(path string) func(string) (string, error) {
	return func(string) (string, error) { return path, nil }
}

func TestDeveloperGit(t *testing.T) {
	dir, git := developerDir(t, "", 0o755)
	xcode, xcodeGit := developerDir(t, filepath.Join("Contents", "Developer"), 0o755)
	tools, toolsGit := developerDir(t, "", 0o755)
	notExecutable, _ := developerDir(t, "", 0o644)
	empty := t.TempDir()

	link := filepath.Join(t.TempDir(), "xcode_select_link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	relativeLink := filepath.Join(filepath.Dir(dir), "xcode_select_link")
	if err := os.Symlink(filepath.Base(dir), relativeLink); err != nil {
		t.Fatal(err)
	}
	noLink := filepath.Join(t.TempDir(), "xcode_select_link")
	defaults := []string{empty, tools}
	shim := found("/usr/bin/git")

	for _, tc := range []struct {
		name         string
		lookPath     func(string) (string, error)
		developerDir string
		link         string
		want         string
	}{
		{"git of another install", found("/opt/homebrew/bin/git"), dir, link, "git"},
		{"no git in PATH", func(string) (string, error) { return "", exec.ErrNotFound }, dir, link, "git"},
		{"chosen with xcode-select", shim, "", link, git},
		{"chosen with a relative link", shim, "", relativeLink, git},
		{"DEVELOPER_DIR wins over xcode-select", shim, filepath.Join(xcode, "Contents", "Developer"), link, xcodeGit},
		{"DEVELOPER_DIR names Xcode.app", shim, xcode, noLink, xcodeGit},
		{"DEVELOPER_DIR without git", shim, empty, link, "git"},
		{"xcode-select chose a directory without git", shim, "", linkTo(t, empty), "git"},
		{"a git that can't run", shim, notExecutable, noLink, "git"},
		{"nothing chosen", shim, "", noLink, toolsGit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := developerGit(tc.lookPath, tc.developerDir, tc.link, defaults); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
	if got := developerGit(shim, "", noLink, []string{empty}); got != "git" {
		t.Fatalf("without developer tools got %q, want the shim", got)
	}
}

// linkTo returns an xcode-select link to dir
func linkTo(t *testing.T, dir string) string {
	t.Helper()
	link := filepath.Join(t.TempDir(), "xcode_select_link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	return link
}

// The git found on this machine runs, whichever it is
func TestFindGit(t *testing.T) {
	git := findGit()
	out, err := exec.Command(git, "--version").Output()
	if err != nil {
		t.Fatalf("%s: %v", git, err)
	}
	t.Logf("%s: %s", git, out)
}
