package git

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	// ErrDetachedHead is returned when HEAD is not on a branch
	ErrDetachedHead Error = "HEAD is detached, check out a branch to push it"
	// ErrNoCommits is returned when the checked out branch has no commits
	ErrNoCommits Error = "the branch has no commits to push"
	// ErrNoRemote is returned when there is no remote to push to
	ErrNoRemote Error = "there is no remote to push to, add one with \"git remote add\""
)

// PushTarget describes what "git push" does with the checked out branch
type PushTarget struct {
	Branch string
	Remote string
	// RemoteBranch is the remote-tracking branch the push updates, e.g. origin/main
	RemoteBranch string
	// NewBranch is set if the remote doesn't have the branch yet, as far as
	// its remote-tracking branches tell
	NewBranch bool
	// SetUpstream is set for a branch without an upstream, the branch it is
	// pushed to becomes its upstream
	SetUpstream bool
	// explicit is set if push.default doesn't push the branch to RemoteBranch,
	// e.g. if its upstream has another name, git push is told where to push
	explicit bool
	// Ahead is the number of commits the remote doesn't have
	Ahead int
	// Commits are the newest of the commits the remote doesn't have
	Commits []*Commit
	// Behind is the number of commits of the remote branch the local one doesn't have
	Behind int
}

// maxPushCommits limits the commits PushTarget lists
const maxPushCommits = 10

// Args returns the arguments of the git push command
func (t *PushTarget) Args() []string {
	refspec := "refs/heads/" + t.Branch + ":refs/heads/" + t.Branch
	switch {
	case t.SetUpstream:
		return []string{"push", "--set-upstream", t.Remote, refspec}
	case t.explicit:
		return []string{"push", t.Remote, refspec}
	}
	return []string{"push"}
}

// PushTarget finds out where "git push" sends the checked out branch and
// which commits it sends
func (r *Repository) PushTarget() (*PushTarget, error) {
	out, err := r.Output("symbolic-ref", "--quiet", "--short", "HEAD")
	if ExitCode(err) == 1 {
		return nil, ErrDetachedHead
	}
	if err != nil {
		return nil, err
	}
	t := &PushTarget{Branch: strings.TrimSpace(string(out))}
	if _, err := r.Output("rev-parse", "--verify", "--quiet", "HEAD"); err != nil {
		return nil, ErrNoCommits
	}

	// the remote git push uses, also for a branch without upstream if
	// remote.pushDefault or branch.<name>.pushRemote is set, and the
	// remote-tracking branch it updates, empty if push.default can't tell
	out, err = r.Output("for-each-ref", "--format=%(upstream)%00%(push:remotename)%00%(push:short)", "refs/heads/"+t.Branch)
	if err != nil {
		return nil, err
	}
	f := strings.Split(strings.TrimSuffix(string(out), "\n"), "\x00")
	if len(f) != 3 {
		return nil, fmt.Errorf("unexpected output of git for-each-ref: %q", out)
	}
	upstream := f[0]
	t.Remote, t.RemoteBranch = f[1], f[2]
	if t.Remote == "" {
		if t.Remote, err = r.defaultRemote(); err != nil {
			return nil, err
		}
	}
	t.SetUpstream = upstream == ""
	if t.RemoteBranch == "" {
		// e.g. the upstream has another name, a new branch, or a push remote
		// other than the upstream's: push to a branch with the same name
		t.explicit = true
		t.RemoteBranch = t.Remote + "/" + t.Branch
	}
	t.NewBranch = !r.refExists("refs/remotes/" + t.RemoteBranch)

	// commits the remote doesn't have, compared to the remote branch if it exists
	unpushed := []string{"HEAD", "--not", "--remotes=" + t.Remote}
	if !t.NewBranch {
		unpushed = []string{t.RemoteBranch + "..HEAD"}
		if t.Behind, err = r.count("HEAD.." + t.RemoteBranch); err != nil {
			return nil, err
		}
	}
	if t.Ahead, err = r.count(unpushed...); err != nil {
		return nil, err
	}
	if t.Commits, err = r.Log(append([]string{fmt.Sprintf("--max-count=%d", maxPushCommits)}, unpushed...)...); err != nil {
		return nil, err
	}
	return t, nil
}

// defaultRemote returns the remote a branch without upstream and push remote
// is pushed to: origin or the only remote
func (r *Repository) defaultRemote() (string, error) {
	out, err := r.Output("remote")
	if err != nil {
		return "", err
	}
	remotes := lines(out)
	for _, remote := range remotes {
		if remote == "origin" {
			return remote, nil
		}
	}
	switch len(remotes) {
	case 0:
		return "", ErrNoRemote
	case 1:
		return remotes[0], nil
	default:
		return "", fmt.Errorf("the branch has no upstream and there are several remotes (%s), push it once with \"git push --set-upstream <remote> <branch>\"",
			strings.Join(remotes, ", "))
	}
}

func (r *Repository) refExists(ref string) bool {
	_, err := r.Output("show-ref", "--verify", "--quiet", ref)
	return err == nil
}

// count returns the number of commits git rev-list lists for args
func (r *Repository) count(args ...string) (int, error) {
	out, err := r.Output(append([]string{"rev-list", "--count"}, args...)...)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}

// Log returns the commits "git log <args>" lists
func (r *Repository) Log(args ...string) ([]*Commit, error) {
	out, err := r.Output(append(append([]string{"log", "-z", "--no-show-signature", commitFormat}, args...), "--")...)
	if err != nil {
		return nil, err
	}
	fields := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	if len(fields) == 1 && fields[0] == "" {
		return nil, nil
	}
	if len(fields)%commitFields != 0 {
		return nil, errors.New("unexpected git log output")
	}
	var commits []*Commit
	for i := 0; i < len(fields); i += commitFields {
		var f [commitFields]string
		copy(f[:], fields[i:i+commitFields])
		commits = append(commits, r.parseCommit(f))
	}
	return commits, nil
}
