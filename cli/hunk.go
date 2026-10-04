package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/isacikgoz/gia/editor"
)

// HunkEditorCommand is the hidden command gitin runs itself to show the
// hunk editor
const HunkEditorCommand = "hunk-editor"

// runHunkEditor lets the user pick hunks of a single file diff and returns
// the patches of the picked ones. The editor runs in a child process: its
// terminal library leaves a goroutine behind every run, which steals key
// presses from the next one, after a few runs the editor stops responding.
func runHunkEditor(diff []byte) ([]string, error) {
	dir, err := os.MkdirTemp("", "gitin-hunks")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	diffPath, patchesPath := filepath.Join(dir, "diff"), filepath.Join(dir, "patches.json")
	if err := os.WriteFile(diffPath, diff, 0o600); err != nil {
		return nil, err
	}

	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	cmd := exec.Command(exe, HunkEditorCommand, diffPath, patchesPath)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("hunk editor: %v %s", err, strings.TrimSpace(stderr.String()))
	}

	data, err := os.ReadFile(patchesPath)
	if err != nil {
		return nil, err
	}
	var patches []string
	return patches, json.Unmarshal(data, &patches)
}

// RunHunkEditor shows the hunk editor for the diff in diffPath and writes
// the patches of the picked hunks to patchesPath as a JSON array
func RunHunkEditor(diffPath, patchesPath string) error {
	diff, err := os.ReadFile(diffPath)
	if err != nil {
		return err
	}
	file, err := parseDiffFile(diff, diffPath)
	if err != nil {
		return err
	}
	ed, err := editor.NewEditor(file)
	if err != nil {
		return err
	}
	patches, err := ed.Run()
	if err != nil {
		return err
	}
	data, err := json.Marshal(patches)
	if err != nil {
		return err
	}
	return os.WriteFile(patchesPath, data, 0o600)
}
