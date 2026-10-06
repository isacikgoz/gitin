package git

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/isacikgoz/gitin/internal/gittest"
)

func pushTarget(t *testing.T, dir string) *PushTarget {
	t.Helper()
	target, err := open(t, dir).PushTarget()
	if err != nil {
		t.Fatal(err)
	}
	return target
}

func summaries(commits []*Commit) string {
	var out []string
	for _, c := range commits {
		out = append(out, c.Summary)
	}
	return strings.Join(out, ",")
}

func TestPushTarget(t *testing.T) {
	_, clone := gittest.NewRemote(t)
	gittest.Commit(t, clone, "first", map[string]string{"a": "1"})
	gittest.Commit(t, clone, "second", map[string]string{"a": "2"})

	got := pushTarget(t, clone)
	if got.Branch != "main" || got.Remote != "origin" || got.RemoteBranch != "origin/main" || got.SetUpstream {
		t.Fatalf("got %+v", got)
	}
	if got.Ahead != 2 || got.Behind != 0 || summaries(got.Commits) != "second,first" {
		t.Fatalf("got ahead %d behind %d commits %s", got.Ahead, got.Behind, summaries(got.Commits))
	}
	if args := strings.Join(got.Args(), " "); args != "push" {
		t.Fatalf("got args %q", args)
	}
}

func TestPushTargetUpToDate(t *testing.T) {
	_, clone := gittest.NewRemote(t)
	got := pushTarget(t, clone)
	if got.Ahead != 0 || got.Behind != 0 || len(got.Commits) != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestPushTargetDiverged(t *testing.T) {
	remote, clone := gittest.NewRemote(t)
	other := gittest.Clone(t, remote)
	gittest.Commit(t, other, "theirs 1", map[string]string{"b": "1"})
	gittest.Commit(t, other, "theirs 2", map[string]string{"b": "2"})
	gittest.Git(t, other, "push", "--quiet")
	gittest.Commit(t, clone, "mine", map[string]string{"a": "1"})
	gittest.Git(t, clone, "fetch", "--quiet")

	got := pushTarget(t, clone)
	if got.Ahead != 1 || got.Behind != 2 || summaries(got.Commits) != "mine" {
		t.Fatalf("got ahead %d behind %d commits %s", got.Ahead, got.Behind, summaries(got.Commits))
	}
}

func TestPushTargetNewBranch(t *testing.T) {
	_, clone := gittest.NewRemote(t)
	gittest.Git(t, clone, "checkout", "--quiet", "-b", "topic")
	gittest.Commit(t, clone, "topic work", map[string]string{"t": "1"})

	got := pushTarget(t, clone)
	if got.Branch != "topic" || got.Remote != "origin" || got.RemoteBranch != "origin/topic" || !got.SetUpstream {
		t.Fatalf("got %+v", got)
	}
	// only the commits the remote doesn't have from any branch
	if got.Ahead != 1 || summaries(got.Commits) != "topic work" {
		t.Fatalf("got ahead %d commits %s", got.Ahead, summaries(got.Commits))
	}
	if args := strings.Join(got.Args(), " "); args != "push --set-upstream origin topic" {
		t.Fatalf("got args %q", args)
	}
}

func TestPushTargetListsTheNewestCommits(t *testing.T) {
	_, clone := gittest.NewRemote(t)
	for i := range 15 {
		gittest.Commit(t, clone, fmt.Sprintf("commit %d", i), nil)
	}
	got := pushTarget(t, clone)
	if got.Ahead != 15 || len(got.Commits) != maxPushCommits || got.Commits[0].Summary != "commit 14" {
		t.Fatalf("got ahead %d, %d commits from %s", got.Ahead, len(got.Commits), got.Commits[0].Summary)
	}
}

func TestPushTargetRemote(t *testing.T) {
	newBranch := func(t *testing.T) string {
		dir := gittest.NewRepo(t)
		gittest.Commit(t, dir, "first", nil)
		return dir
	}

	t.Run("no remote", func(t *testing.T) {
		if _, err := open(t, newBranch(t)).PushTarget(); !errors.Is(err, ErrNoRemote) {
			t.Fatalf("got error %v", err)
		}
	})

	t.Run("the only remote", func(t *testing.T) {
		dir := newBranch(t)
		gittest.Git(t, dir, "remote", "add", "upstream", "https://example.com/repo.git")
		if got := pushTarget(t, dir); got.Remote != "upstream" || got.RemoteBranch != "upstream/main" || got.Ahead != 1 {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("several remotes", func(t *testing.T) {
		dir := newBranch(t)
		gittest.Git(t, dir, "remote", "add", "fork", "https://example.com/fork.git")
		gittest.Git(t, dir, "remote", "add", "upstream", "https://example.com/repo.git")
		_, err := open(t, dir).PushTarget()
		if err == nil || !strings.Contains(err.Error(), "several remotes (fork, upstream)") {
			t.Fatalf("got error %v", err)
		}

		gittest.Git(t, dir, "config", "remote.pushDefault", "fork")
		if got := pushTarget(t, dir); got.Remote != "fork" {
			t.Fatalf("got %+v", got)
		}

		gittest.Git(t, dir, "remote", "add", "origin", "https://example.com/origin.git")
		gittest.Git(t, dir, "config", "--unset", "remote.pushDefault")
		if got := pushTarget(t, dir); got.Remote != "origin" {
			t.Fatalf("got %+v", got)
		}
	})
}

func TestPushTargetErrors(t *testing.T) {
	_, clone := gittest.NewRemote(t)
	gittest.Git(t, clone, "checkout", "--quiet", "--detach")
	if _, err := open(t, clone).PushTarget(); !errors.Is(err, ErrDetachedHead) {
		t.Fatalf("detached HEAD: got error %v", err)
	}
	if _, err := open(t, gittest.NewRepo(t)).PushTarget(); !errors.Is(err, ErrNoCommits) {
		t.Fatalf("no commits: got error %v", err)
	}
}

func TestLog(t *testing.T) {
	dir := gittest.NewRepo(t)
	gittest.Commit(t, dir, "first", nil)
	gittest.Commit(t, dir, "second\n\nbody", nil)
	r := open(t, dir)

	commits, err := r.Log("--max-count=5")
	if err != nil || summaries(commits) != "second,first" {
		t.Fatalf("got %s, %v", summaries(commits), err)
	}
	if commits, err = r.Log("HEAD..HEAD"); err != nil || len(commits) != 0 {
		t.Fatalf("got %v, %v for an empty range", commits, err)
	}
	if _, err := r.Log("no-such-revision"); err == nil {
		t.Fatal("no error for an unknown revision")
	}
}
