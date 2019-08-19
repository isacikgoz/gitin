package cli

import (
	"os"
	"os/exec"

	"github.com/isacikgoz/gitin/git"
)

func popGitCommand(r *git.Repository, args []string) error {
	os.Setenv("LESS", "-RCS")
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Path()

	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin

	if err := cmd.Start(); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}
