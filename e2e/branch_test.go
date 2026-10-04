package e2e

import (
	"strings"
	"testing"

	"github.com/isacikgoz/gitin/internal/gittest"
)

func branchRepo(t *testing.T) string {
	t.Helper()
	upstream := gittest.NewRepo(t)
	gittest.Commit(t, upstream, "base", map[string]string{"a": "1"})
	dir := gittest.Clone(t, upstream)
	gittest.Git(t, dir, "branch", "merged")
	gittest.Git(t, dir, "checkout", "--quiet", "-b", "unmerged")
	gittest.Commit(t, dir, "unmerged work", map[string]string{"a": "2"})
	gittest.Git(t, dir, "checkout", "--quiet", "main")
	return dir
}

func TestBranchListAndCheckout(t *testing.T) {
	dir := branchRepo(t)
	s := start(t, dir, []string{"branch"})
	s.waitFrame("Branches", 0, "> main *", "merged", "unmerged", "origin/main",
		"Last commit was", "This branch is up to date with origin/main.")

	s.send(down, down, enter)
	if code := s.wait(); code != 0 {
		t.Fatalf("got exit code %d", code)
	}
	if got := gittest.Git(t, dir, "rev-parse", "--abbrev-ref", "HEAD"); got != "unmerged" {
		t.Fatalf("got branch %s", got)
	}
}

func TestBranchCheckoutFailure(t *testing.T) {
	dir := branchRepo(t)
	gittest.WriteFile(t, dir, "a", "local change")
	s := start(t, dir, []string{"branch"})
	s.waitFrame("Branches", 0, "unmerged")
	m := s.mark()
	s.send(down, down, enter)
	s.waitFrame("Branches", m, "would be overwritten by checkout")
	if !s.running() {
		t.Fatal("gitin exited after a failed checkout")
	}
}

func TestBranchDelete(t *testing.T) {
	dir := branchRepo(t)
	s := start(t, dir, []string{"branch"})
	s.waitFrame("Branches", 0, "merged")

	m := s.mark()
	s.send(down, "d")
	frame := s.waitFrame("Branches", m, "> unmerged")
	if strings.Contains(frame, " merged\n") {
		t.Fatalf("merged branch is still listed:\n%s", frame)
	}

	m = s.mark()
	s.send("d")
	s.waitFrame("Branches", m, "not fully merged")
	m = s.mark()
	s.send("D")
	frame = s.waitFrame("Branches", m, "origin/main")
	if strings.Contains(frame, "unmerged") {
		t.Fatalf("unmerged branch is still listed:\n%s", frame)
	}
	if got := gittest.Git(t, dir, "branch", "--format=%(refname:short)"); got != "main" {
		t.Fatalf("got branches %q", got)
	}
}
