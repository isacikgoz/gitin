package cli

import (
	"os"
	"os/exec"

	"github.com/isacikgoz/gitin/git"
)

type CommitOptions struct {
	MinCommitLength int
	MaxCommitLength int
	Message         string
	PromptOps       *PromptOptions
}

func CommitBuilder(r *git.Repository, opts *CommitOptions) error {
	if len(opts.Message) <= 0 {
		opts.Message = "message"
	}
	return commitPrompt(r, opts)
}

func commitPrompt(r *git.Repository, opts *CommitOptions) error {
	args := []string{"--edit", "--quiet"}
	return execCommit(r, args)
}

func commitAmend(r *git.Repository) error {
	args := []string{"--amend", "--quiet"}
	return execCommit(r, args)
}

func execCommit(r *git.Repository, args []string) error {
	args = append([]string{"commit"}, args...)
	cmd := exec.Command("git", args...)
	cmd.Dir = r.AbsPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	ok := false
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := cmd.Wait(); err == nil {
		ok = true
	}
	if err := r.InitializeStatus(); err != nil {
		return err
	}
	if ok {
		if err := popGitCmd(r, r.LastCommitArgs()); err != nil {
			return err
		}
	}
	return NoErrRecurse
}
