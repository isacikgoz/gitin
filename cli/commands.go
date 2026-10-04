package cli

import (
	"io"
	"os"

	"github.com/isacikgoz/gitin/git"
)

// the terminal interactive git commands use
var (
	stdin  io.Reader = os.Stdin
	stdout io.Writer = os.Stdout
)

// popGitCommand runs an interactive git command, e.g. one that opens a pager
// or an editor, on the terminal. Failures return git's error message.
func popGitCommand(r *git.Repository, args []string) error {
	cmd := r.Command(args...)
	// the pager must not exit on its own, the prompt would overwrite it
	cmd.Env = append(os.Environ(), "LESS=-RCS")
	cmd.Stdout = stdout
	cmd.Stdin = stdin
	_, err := git.Run(cmd)
	return err
}

// runGitCommand runs a non-interactive git command. Failures return git's
// error message.
func runGitCommand(r *git.Repository, args []string) error {
	_, err := r.Output(args...)
	return err
}
