package cli

import (
	"io"
	"os"

	"github.com/isacikgoz/gitin/git"
	"github.com/isacikgoz/gitin/prompt"
)

// the terminal interactive commands use
var (
	stdin  io.Reader = os.Stdin
	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr
)

// popGitCommand runs a git command that shows something, e.g. a diff in a
// pager. The prompt steps aside and is drawn again below the output, the
// output stays on the screen. Failures return git's error message.
func popGitCommand(p *prompt.Prompt, r *git.Repository, args []string) error {
	return p.Suspend(func() error { return terminalGitCommand(r, args) })
}

// terminalGitCommand runs a git command with the terminal, e.g. it can page
// its output or open an editor. Call it while the prompt is suspended.
func terminalGitCommand(r *git.Repository, args []string) error {
	cmd := r.Command(args...)
	// less shows the output on its own screen until the user quits it
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
