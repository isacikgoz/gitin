package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	git "github.com/isacikgoz/libgit2-api"
	"github.com/waigani/diffparser"
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

// fileStatArgs returns git command args for getting diff
func fileStatArgs(e *git.StatusEntry) []string {
	var args []string
	if e.Indexed() {
		args = []string{"diff", "--cached", e.String()}
	} else if e.EntryType == git.StatusEntryTypeUntracked {
		args = []string{"diff", "--no-index", "/dev/null", e.String()}
	} else {
		args = []string{"diff", "--", e.String()}
	}
	return args
}

// lastCommitArgs returns the args for show stat
func lastCommitArgs(r *git.Repository) ([]string, error) {
	r.LoadStatus()
	head := r.Head
	if head == nil {
		return nil, fmt.Errorf("can't get HEAD")
	}
	hash := string(head.Target().Hash)
	args := []string{"show", "--stat", hash}
	return args, nil
}

func generateDiffFile(r *git.Repository, entry *git.StatusEntry) (*diffparser.DiffFile, error) {
	args := fileStatArgs(entry)
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Path()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	diff, err := diffparser.Parse(string(out))
	if err != nil {
		return nil, err
	}
	return diff.Files[0], nil
}

func applyPatchCmd(r *git.Repository, entry *git.StatusEntry, patch string) error {
	mode := []string{"apply", "--cached"}
	if entry.Indexed() {
		mode = []string{"apply", "--cached", "--reverse"}
	}
	cmd := exec.Command("git", mode...)
	cmd.Dir = r.Path()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	go func() {
		defer stdin.Close()
		io.WriteString(stdin, patch+"\n")
	}()
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
