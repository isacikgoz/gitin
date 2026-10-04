package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Commit is a commit of the repository
type Commit struct {
	owner *Repository

	Hash    string
	Parents []string
	Author  *Signature
	Summary string
}

// Signature is the person who signs a commit
type Signature struct {
	Name  string
	Email string
	When  time.Time
}

// commitFormat prints the fields of a commit separated by NUL, -z ends
// every commit with NUL as well.
const commitFormat = "--format=%H%x00%P%x00%aN%x00%aE%x00%at%x00%s"

const commitFields = 6

// Commits streams the commits reachable from HEAD, newest first, as git log
// lists them. The channel is closed when all commits are sent, after that
// wait reports whether git failed. Cancel ctx to stop early.
func (r *Repository) Commits(ctx context.Context) (commits <-chan *Commit, wait func() error) {
	out := make(chan *Commit, 1024)
	errc := make(chan error, 1)
	go func() {
		defer close(out)
		errc <- r.streamCommits(ctx, out)
	}()
	return out, func() error { return <-errc }
}

func (r *Repository) streamCommits(ctx context.Context, out chan<- *Commit) error {
	cmd := exec.CommandContext(ctx, "git", "log", "-z", "--no-show-signature", commitFormat, "--")
	cmd.Dir = r.path
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	fail := func(err error) error {
		_, _ = io.Copy(io.Discard, stdout)
		if werr := cmd.Wait(); werr != nil {
			err = &CommandError{Args: cmd.Args[1:], Stderr: stderr.String(), Err: werr}
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 64*1024*1024)
	scanner.Split(scanNUL)
	var fields [commitFields]string
	for {
		for i := range fields {
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					return fail(err)
				}
				if i != 0 {
					return fail(fmt.Errorf("git log output ended within a commit"))
				}
				return fail(nil)
			}
			fields[i] = scanner.Text()
		}
		select {
		case out <- r.parseCommit(fields):
		case <-ctx.Done():
			return fail(ctx.Err())
		}
	}
}

func (r *Repository) parseCommit(fields [commitFields]string) *Commit {
	c := &Commit{
		owner:   r,
		Hash:    fields[0],
		Author:  &Signature{Name: fields[2], Email: fields[3]},
		Summary: fields[5],
	}
	if fields[1] != "" {
		c.Parents = strings.Split(fields[1], " ")
	}
	if ts, err := strconv.ParseInt(fields[4], 10, 64); err == nil {
		c.Author.When = time.Unix(ts, 0)
	}
	return c
}

// scanNUL is a bufio.SplitFunc for NUL terminated records
func scanNUL(data []byte, atEOF bool) (int, []byte, error) {
	if i := bytes.IndexByte(data, 0); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func (c *Commit) String() string {
	return c.Summary
}

// ParentID returns the hash of the first parent
func (c *Commit) ParentID() (string, error) {
	if len(c.Parents) == 0 {
		return "", fmt.Errorf("commit does not have parents")
	}
	return c.Parents[0], nil
}

// Diff lists the files changed by the commit compared to its first parent,
// like "git show --first-parent <commit>" does.
func (c *Commit) Diff() (*Diff, error) {
	args := []string{"diff-tree", "-r", "-z", "--raw", "--numstat", "--no-renames", "--no-commit-id"}
	if parent, err := c.ParentID(); err == nil {
		args = append(args, parent, c.Hash)
	} else {
		args = append(args, "--root", c.Hash)
	}
	out, err := c.owner.Output(args...)
	if err != nil {
		return nil, err
	}
	deltas, err := parseDiffTree(out)
	if err != nil {
		return nil, err
	}
	for _, d := range deltas {
		d.Commit = c
	}
	return &Diff{deltas: deltas}, nil
}

// parseDiffTree parses the output of "diff-tree -z --raw --numstat". Raw
// records come first, ":<modes> <hashes> <status>\0<path>\0" each, followed
// by the numstat records "<added>\t<deleted>\t<path>\0" in the same order.
func parseDiffTree(out []byte) ([]*DiffDelta, error) {
	records := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	var deltas []*DiffDelta
	i := 0
	for ; i+1 < len(records) && strings.HasPrefix(records[i], ":"); i += 2 {
		meta := strings.Fields(records[i])
		if len(meta) != 5 || meta[4] == "" {
			return nil, fmt.Errorf("unexpected diff-tree record %q", records[i])
		}
		path := records[i+1]
		deltas = append(deltas, &DiffDelta{
			Status:  deltaStatusFromCode(meta[4][0]),
			OldFile: &DiffFile{Path: path, Hash: meta[2]},
			NewFile: &DiffFile{Path: path, Hash: meta[3]},
		})
	}
	stats := records[i:]
	if len(stats) == 1 && stats[0] == "" {
		stats = nil
	}
	if len(stats) != len(deltas) {
		return nil, fmt.Errorf("diff-tree listed %d files but %d line counts", len(deltas), len(stats))
	}
	for j, stat := range stats {
		f := strings.SplitN(stat, "\t", 3)
		if len(f) != 3 || f[2] != deltas[j].NewFile.Path {
			return nil, fmt.Errorf("unexpected diff-tree record %q", stat)
		}
		if f[0] == "-" {
			deltas[j].Binary = true
			continue
		}
		deltas[j].Additions, _ = strconv.Atoi(f[0])
		deltas[j].Deletions, _ = strconv.Atoi(f[1])
	}
	return deltas, nil
}
