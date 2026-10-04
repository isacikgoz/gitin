package cli

import (
	"strings"
	"testing"

	"github.com/isacikgoz/gitin/git"
	"github.com/isacikgoz/gitin/internal/gittest"
)

// branchRepo returns a clone with a merged, an unmerged and a tracking branch
func branchRepo(t *testing.T) string {
	t.Helper()
	upstream := gittest.NewRepo(t)
	gittest.Commit(t, upstream, "base", map[string]string{"a": "1"})
	gittest.Git(t, upstream, "branch", "shared")
	dir := gittest.Clone(t, upstream)
	gittest.Git(t, dir, "branch", "merged")
	gittest.Git(t, dir, "checkout", "--quiet", "-b", "unmerged")
	gittest.Commit(t, dir, "unmerged work", map[string]string{"a": "2"})
	gittest.Git(t, dir, "checkout", "--quiet", "main")
	return dir
}

func newTestBranch(t *testing.T, dir string) *branch {
	t.Helper()
	b, err := newBranch(open(t, dir), opts)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func findBranch(t *testing.T, b *branch, name string) *git.Branch {
	t.Helper()
	items, _ := b.prompt.State().List.Items()
	for b.prompt.State().List.CanPageDown() {
		b.prompt.State().List.PageDown()
		more, _ := b.prompt.State().List.Items()
		items = append(items, more...)
	}
	b.prompt.State().List.SetCursor(0)
	for _, item := range items {
		if br := item.(*git.Branch); br.Name == name {
			return br
		}
	}
	t.Fatalf("branch %s is not listed", name)
	return nil
}

func currentBranch(t *testing.T, dir string) string {
	t.Helper()
	return gittest.Git(t, dir, "rev-parse", "--abbrev-ref", "HEAD")
}

func TestBranchPrompt(t *testing.T) {
	p, err := BranchPrompt(open(t, branchRepo(t)), opts)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(visible(p), " "); got != "main merged unmerged origin/HEAD origin/main" {
		t.Fatalf("got %s", got)
	}
}

func TestBranchCheckout(t *testing.T) {
	dir := branchRepo(t)
	b := newTestBranch(t, dir)
	if err := b.onSelect(findBranch(t, b, "unmerged")); err != nil {
		t.Fatal(err)
	}
	if got := currentBranch(t, dir); got != "unmerged" {
		t.Fatalf("got branch %s", got)
	}
}

func TestBranchCheckoutWithConflictingChanges(t *testing.T) {
	dir := branchRepo(t)
	gittest.WriteFile(t, dir, "a", "local change")
	b := newTestBranch(t, dir)
	err := b.onSelect(findBranch(t, b, "unmerged"))
	if err == nil || !strings.Contains(err.Error(), "would be overwritten") {
		t.Fatalf("got error %v", err)
	}
	if got := currentBranch(t, dir); got != "main" {
		t.Fatalf("got branch %s", got)
	}
}

func TestBranchCheckoutOfBranchNamedLikeAFile(t *testing.T) {
	dir := branchRepo(t)
	gittest.Git(t, dir, "branch", "a")
	b := newTestBranch(t, dir)
	if err := b.onSelect(findBranch(t, b, "a")); err != nil {
		t.Fatal(err)
	}
	if got := currentBranch(t, dir); got != "a" {
		t.Fatalf("got branch %s", got)
	}
}

func TestBranchDelete(t *testing.T) {
	dir := branchRepo(t)
	b := newTestBranch(t, dir)

	if err := b.deleteBranch(findBranch(t, b, "merged")); err != nil {
		t.Fatal(err)
	}
	err := b.deleteBranch(findBranch(t, b, "unmerged"))
	if err == nil || !strings.Contains(err.Error(), "not fully merged") {
		t.Fatalf("deleting an unmerged branch: got error %v", err)
	}
	if err := b.forceDeleteBranch(findBranch(t, b, "unmerged")); err != nil {
		t.Fatal(err)
	}
	if err := b.deleteBranch(findBranch(t, b, "origin/shared")); err != nil {
		t.Fatal(err)
	}
	want := "main origin/HEAD origin/main"
	refs := gittest.Git(t, dir, "for-each-ref", "--format=%(refname:lstrip=2)", "refs/heads", "refs/remotes")
	if got := strings.Join(strings.Fields(refs), " "); got != want {
		t.Fatalf("got branches %q", got)
	}
	if got := strings.Join(visible(b.prompt), " "); got != want {
		t.Fatalf("list was not reloaded: %s", got)
	}
}

func TestBranchInfoOfPrompt(t *testing.T) {
	dir := branchRepo(t)
	gittest.Git(t, dir, "branch", "--set-upstream-to=origin/main", "merged")
	b := newTestBranch(t, dir)

	got := lines(b.branchInfo(findBranch(t, b, "merged")))
	if len(got) != 2 || !strings.HasPrefix(got[0], "Last commit was ") ||
		got[1] != "This branch is up to date with origin/main." {
		t.Fatalf("got %q", got)
	}
	got = lines(b.branchInfo(findBranch(t, b, "origin/main")))
	if len(got) != 1 || !strings.HasPrefix(got[0], "Last commit was ") {
		t.Fatalf("got %q for a remote branch", got)
	}
	if got := b.branchInfo(&git.Branch{Name: "no commit"}); len(got) != 0 {
		t.Fatalf("got %q for a branch without a date", lines(got))
	}
}

func TestQuit(t *testing.T) {
	dir := branchRepo(t)
	b := newTestBranch(t, dir)
	if err := b.quit(nil); err != nil {
		t.Fatal(err)
	}
	s := newTestStatus(t, statusRepo(t))
	if err := s.quit(nil); err != nil {
		t.Fatal(err)
	}
	l := newTestLog(t, logRepo(t))
	if err := l.quit(selected(t, l.prompt)); err != nil {
		t.Fatal(err)
	}
}
