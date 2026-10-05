// Package git reads repository data by running the git command line tool with
// machine readable output formats. It requires git 2.18 or newer.
package git

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Repository is the main interface to a git repository
type Repository struct {
	path string
}

// RefType defines the ref types
type RefType uint8

// These types are used for mapping references
const (
	RefTypeTag RefType = iota
	RefTypeBranch
	RefTypeHEAD
)

// Ref is a named reference to a commit, e.g. a branch or a tag
type Ref interface {
	Type() RefType
	String() string
}

// CommandError is returned when a git command fails, it carries the message
// git printed to its standard error.
type CommandError struct {
	Args   []string
	Stderr string
	Err    error
}

func (e *CommandError) Error() string {
	msg := strings.TrimSpace(e.Stderr)
	if msg == "" {
		return fmt.Sprintf("git %s: %v", strings.Join(e.Args, " "), e.Err)
	}
	return strings.TrimPrefix(msg, "fatal: ")
}

func (e *CommandError) Unwrap() error {
	return e.Err
}

// Open finds the repository containing path the same way git does. Commands
// run from the root of the working tree, or from the git directory if the
// repository has no working tree (e.g. a bare repository).
func Open(path string) (*Repository, error) {
	r := &Repository{path: path}
	// without a working tree, older git versions print nothing instead of failing
	out, err := r.Output("rev-parse", "--show-toplevel")
	if err != nil || len(out) == 0 {
		out, err = r.Output("rev-parse", "--absolute-git-dir")
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCannotOpenRepo, err)
	}
	r.path = strings.TrimSuffix(string(out), "\n")
	return r, nil
}

// Merging reports whether a merge is in progress
func (r *Repository) Merging() bool {
	_, err := r.Output("rev-parse", "--quiet", "--verify", "MERGE_HEAD")
	return err == nil
}

// Path returns the directory the git commands run in
func (r *Repository) Path() string {
	return r.path
}

// Command returns a git command that runs in the repository
func (r *Repository) Command(args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.path
	return cmd
}

// Output runs a git command in the repository and returns its standard output
func (r *Repository) Output(args ...string) ([]byte, error) {
	return Run(r.Command(args...))
}

// Run runs cmd and returns its standard output, also when it fails. Standard
// output and standard error are captured unless they are already set. If the
// command fails, the error is a *CommandError with the message git printed.
func Run(cmd *exec.Cmd) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	if cmd.Stdout == nil {
		cmd.Stdout = &stdout
	}
	if cmd.Stderr == nil {
		cmd.Stderr = &stderr
	}
	if err := cmd.Run(); err != nil {
		return stdout.Bytes(), &CommandError{Args: cmd.Args[1:], Stderr: stderr.String(), Err: err}
	}
	return stdout.Bytes(), nil
}

// ExitCode returns the exit code of a failed git command, or -1 if err is
// not the result of git exiting unsuccessfully.
func ExitCode(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}
