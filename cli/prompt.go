package cli

import (
	"errors"
	"os"
	"os/exec"

	"github.com/isacikgoz/gitin/git"
)

// PromptOptions is the common options for building a prompt
type PromptOptions struct {
	Cursor   int
	Scroll   int
	Size     int
	HideHelp bool
}

var (
	// NoErrRecurse is just a indactor to the caller to pop prompt back
	NoErrRecurse error = errors.New("catch")
)

func popGitCmd(r *git.Repository, args []string) error {
	os.Setenv("LESS", "-RCS")
	cmd := exec.Command("git", args...)
	cmd.Dir = r.AbsPath

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Start(); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	return NoErrRecurse
}
