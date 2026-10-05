package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// unstagedPatch is the file in the git directory that keeps unstaged
// changes while they are set aside
const unstagedPatch = "gitin-unstaged.patch"

// UnstagedChanges are the unstaged changes of tracked files, set aside so
// the working tree has what the next commit has
type UnstagedChanges struct {
	r *Repository
	// patch is the file with the changes, empty if there were none
	patch string
	files []string
}

// unstagedDiff are the arguments of git diff showing the unstaged changes,
// independent of the user's diff settings. Intent-to-add files show up as
// added files, the only ones a diff to the index can have.
var unstagedDiff = []string{"diff", "--ita-invisible-in-index", "--no-renames", "--ignore-submodules", "--no-color", "--no-ext-diff", "--no-textconv"}

// SetAsideUnstaged removes the unstaged changes of tracked files from the
// working tree and keeps them in the git directory until Restore. Untracked
// and intent-to-add files stay.
func (r *Repository) SetAsideUnstaged() (*UnstagedChanges, error) {
	gitDir, err := r.Output("rev-parse", "--absolute-git-dir")
	if err != nil {
		return nil, err
	}
	path := filepath.Join(strings.TrimSpace(string(gitDir)), unstagedPatch)
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("unstaged changes set aside earlier are still in %s, restore them with \"git apply %s\" and delete the file", path, path)
	}

	changes, err := r.Output(append(unstagedDiff, "--name-status", "-z")...)
	if err != nil {
		return nil, err
	}
	u := &UnstagedChanges{r: r}
	// intent-to-add files are left alone like untracked ones, setting them
	// aside would empty them
	pathspec := []string{"--", ":/"}
	records := strings.Split(strings.TrimSuffix(string(changes), "\x00"), "\x00")
	for i := 0; i+1 < len(records); i += 2 {
		if status, name := records[i], records[i+1]; status == "A" {
			pathspec = append(pathspec, ":(exclude,literal)"+name)
		} else {
			u.files = append(u.files, name)
		}
	}
	if len(u.files) == 0 {
		return u, nil
	}

	patch, err := r.Output(append(append(unstagedDiff, "--binary", "--src-prefix=a/", "--dst-prefix=b/"), pathspec...)...)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, patch, 0o600); err != nil {
		return nil, err
	}
	u.patch = path
	if err := u.checkoutIndex(); err != nil {
		return nil, fmt.Errorf("could not set aside the unstaged changes, they are in %s: %w", path, err)
	}
	return u, nil
}

// Any reports whether there were unstaged changes to set aside
func (u *UnstagedChanges) Any() bool {
	return u.patch != ""
}

// Restore puts the unstaged changes back. If the working tree changed in the
// meantime in a way that conflicts with them, e.g. a formatter changed the
// same files, these changes are undone first and Restore reports it.
func (u *UnstagedChanges) Restore() (undone bool, err error) {
	if u.patch == "" {
		return false, nil
	}
	apply := func() error {
		_, err := u.r.Output("apply", "--whitespace=nowarn", u.patch)
		return err
	}
	if err := apply(); err != nil {
		if err := u.checkoutIndex(); err != nil {
			return false, u.notRestored(err)
		}
		if err := apply(); err != nil {
			return true, u.notRestored(err)
		}
		undone = true
	}
	return undone, os.Remove(u.patch)
}

func (u *UnstagedChanges) notRestored(err error) error {
	return fmt.Errorf("could not restore the unstaged changes, they are in %s, restore them with \"git apply %s\": %w", u.patch, u.patch, err)
}

// checkoutIndex makes the changed files match the index
func (u *UnstagedChanges) checkoutIndex() error {
	cmd := u.r.Command("checkout-index", "--force", "-z", "--stdin")
	cmd.Stdin = strings.NewReader(strings.Join(u.files, "\x00"))
	_, err := Run(cmd)
	return err
}
