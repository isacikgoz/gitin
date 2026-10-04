package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	// isolate the tests from the user's git configuration
	for k, v := range map[string]string{
		"GIT_CONFIG_GLOBAL":   os.DevNull,
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_AUTHOR_NAME":     "Ada Lovelace",
		"GIT_AUTHOR_EMAIL":    "ada@example.com",
		"GIT_COMMITTER_NAME":  "Ada Lovelace",
		"GIT_COMMITTER_EMAIL": "ada@example.com",
	} {
		if err := os.Setenv(k, v); err != nil {
			panic(err)
		}
	}
	os.Exit(m.Run())
}

// gitRun runs git in dir and returns its trimmed output
func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newRepo creates a repository on branch main without commits
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitRun(t, dir, "init", "--quiet", "--initial-branch=main")
	return dir
}

// commit writes the files and commits them, returning the commit hash
func commit(t *testing.T, dir, message string, files map[string]string) string {
	t.Helper()
	for name, content := range files {
		writeFile(t, dir, name, content)
	}
	gitRun(t, dir, "add", "--all")
	gitRun(t, dir, "commit", "--quiet", "--allow-empty", "-m", message)
	return gitRun(t, dir, "rev-parse", "HEAD")
}

func open(t *testing.T, dir string) *Repository {
	t.Helper()
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func resolved(t *testing.T, path string) string {
	t.Helper()
	p, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestOpen(t *testing.T) {
	t.Run("nested directory opens the working tree root", func(t *testing.T) {
		dir := newRepo(t)
		writeFile(t, dir, "a/b/file", "x")
		r := open(t, filepath.Join(dir, "a", "b"))
		if got, want := resolved(t, r.Path()), resolved(t, dir); got != want {
			t.Fatalf("got path %s, want %s", got, want)
		}
	})

	t.Run("nested repository opens the inner repository", func(t *testing.T) {
		outer := newRepo(t)
		inner := filepath.Join(outer, "inner")
		gitRun(t, outer, "init", "--quiet", "inner")
		r := open(t, inner)
		if got, want := resolved(t, r.Path()), resolved(t, inner); got != want {
			t.Fatalf("got path %s, want %s", got, want)
		}
	})

	t.Run("bare repository opens the git directory", func(t *testing.T) {
		dir := t.TempDir()
		gitRun(t, dir, "init", "--quiet", "--bare", "repo.git")
		r := open(t, filepath.Join(dir, "repo.git"))
		if got, want := resolved(t, r.Path()), resolved(t, filepath.Join(dir, "repo.git")); got != want {
			t.Fatalf("got path %s, want %s", got, want)
		}
	})

	t.Run("not a repository", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
		_, err := Open(dir)
		if !errors.Is(err, ErrCannotOpenRepo) {
			t.Fatalf("got error %v, want %v", err, ErrCannotOpenRepo)
		}
		if !strings.Contains(err.Error(), "not a git repository") {
			t.Fatalf("error %q does not include git's message", err)
		}
	})
}

func collect(t *testing.T, r *Repository) ([]*Commit, error) {
	t.Helper()
	ch, wait := r.Commits(context.Background())
	var commits []*Commit
	for c := range ch {
		commits = append(commits, c)
	}
	return commits, wait()
}

func TestCommits(t *testing.T) {
	dir := newRepo(t)
	root := commit(t, dir, "root commit", map[string]string{"a": "1\n"})
	second := commit(t, dir, "Speed up by 50%", map[string]string{"b": "2\n"})
	third := commit(t, dir, "third", map[string]string{"c": "3\n"})

	commits, err := collect(t, open(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 3 {
		t.Fatalf("got %d commits, want 3", len(commits))
	}
	for i, want := range []string{third, second, root} {
		if commits[i].Hash != want {
			t.Fatalf("commit %d is %s, want %s", i, commits[i].Hash, want)
		}
	}
	c := commits[1]
	if c.Summary != "Speed up by 50%" || c.String() != c.Summary {
		t.Fatalf("got summary %q", c.Summary)
	}
	if c.Author.Name != "Ada Lovelace" || c.Author.Email != "ada@example.com" || c.Author.When.IsZero() {
		t.Fatalf("got author %+v", c.Author)
	}
	if pid, err := c.ParentID(); err != nil || pid != root {
		t.Fatalf("got parent %q, %v, want %s", pid, err, root)
	}
	if _, err := commits[2].ParentID(); err == nil {
		t.Fatal("root commit has a parent")
	}
}

func TestCommitsShallowClone(t *testing.T) {
	dir := newRepo(t)
	for i := range 3 {
		commit(t, dir, fmt.Sprintf("commit %d", i), map[string]string{"a": fmt.Sprint(i)})
	}
	clone := filepath.Join(t.TempDir(), "clone")
	gitRun(t, dir, "clone", "--quiet", "--depth=2", "file://"+dir, clone)

	commits, err := collect(t, open(t, clone))
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 2 {
		t.Fatalf("got %d commits, want 2", len(commits))
	}
}

func TestCommitsWithoutCommits(t *testing.T) {
	_, err := collect(t, open(t, newRepo(t)))
	if err == nil || !strings.Contains(err.Error(), "does not have any commits") {
		t.Fatalf("got error %v", err)
	}
}

func TestCommitsCancel(t *testing.T) {
	dir := newRepo(t)
	// fast-import creates many commits quickly
	var script strings.Builder
	for i := 1; i <= 3000; i++ {
		fmt.Fprintf(&script, "commit refs/heads/main\nmark :%d\ncommitter A <a@b> %d +0000\ndata 5\nc%04d", i, 1000000+i, i)
		if i > 1 {
			fmt.Fprintf(&script, "\nfrom :%d", i-1)
		}
		script.WriteString("\n\n")
	}
	cmd := exec.Command("git", "fast-import", "--quiet")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(script.String())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fast-import: %v\n%s", err, out)
	}

	ctx, cancel := context.WithCancel(context.Background())
	ch, wait := open(t, dir).Commits(ctx)
	<-ch
	cancel()
	for range ch {
	}
	if err := wait(); !errors.Is(err, context.Canceled) {
		t.Fatalf("got error %v, want %v", err, context.Canceled)
	}
}

func TestCommitDiff(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "root", map[string]string{"keep.txt": "a\nb\n", "gone.txt": "x\n", "50% off.txt": "y\n"})
	writeFile(t, dir, "bin.dat", "\x00\x01\x02")
	commit(t, dir, "second", map[string]string{"keep.txt": "a\nc\nd\n", "new.txt": "n\n"})
	gitRun(t, dir, "rm", "--quiet", "gone.txt")
	gitRun(t, dir, "commit", "--quiet", "-m", "remove")

	commits, err := collect(t, open(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	type change struct {
		status     DeltaStatus
		adds, dels int
		binary     bool
	}
	tests := map[string]map[string]change{
		"root": {
			"50% off.txt": {DeltaAdded, 1, 0, false},
			"gone.txt":    {DeltaAdded, 1, 0, false},
			"keep.txt":    {DeltaAdded, 2, 0, false},
		},
		"second": {
			"bin.dat":  {DeltaAdded, 0, 0, true},
			"keep.txt": {DeltaModified, 2, 1, false},
			"new.txt":  {DeltaAdded, 1, 0, false},
		},
		"remove": {
			"gone.txt": {DeltaDeleted, 0, 1, false},
		},
	}
	for _, c := range commits {
		diff, err := c.Diff()
		if err != nil {
			t.Fatal(err)
		}
		want := tests[c.Summary]
		got := make(map[string]change)
		for _, d := range diff.Deltas() {
			got[d.String()] = change{d.Status, d.Additions, d.Deletions, d.Binary}
			if d.Commit != c {
				t.Fatalf("delta %s does not point to its commit", d)
			}
		}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("commit %q: got %v, want %v", c.Summary, got, want)
		}
	}
}

func TestCommitDiffOfMergeComparesFirstParent(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "root", map[string]string{"a": "1\n"})
	gitRun(t, dir, "switch", "--quiet", "-c", "topic")
	commit(t, dir, "topic", map[string]string{"topic.txt": "t\n"})
	gitRun(t, dir, "switch", "--quiet", "main")
	commit(t, dir, "main", map[string]string{"main.txt": "m\n"})
	gitRun(t, dir, "merge", "--quiet", "--no-edit", "topic")

	commits, err := collect(t, open(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	merge := commits[0]
	if len(merge.Parents) != 2 {
		t.Fatalf("got %d parents, want 2", len(merge.Parents))
	}
	diff, err := merge.Diff()
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.Deltas()) != 1 || diff.Deltas()[0].String() != "topic.txt" {
		t.Fatalf("got deltas %v, want [topic.txt]", diff.Deltas())
	}
}

func TestRefs(t *testing.T) {
	dir := newRepo(t)
	first := commit(t, dir, "first", nil)
	gitRun(t, dir, "tag", "--annotate", "--message=release", "v1")
	second := commit(t, dir, "second", nil)
	gitRun(t, dir, "tag", "v2")
	gitRun(t, dir, "branch", "topic", first)

	refs, err := open(t, dir).Refs()
	if err != nil {
		t.Fatal(err)
	}
	describe := func(hash string) []string {
		var out []string
		for _, ref := range refs[hash] {
			out = append(out, fmt.Sprintf("%d:%s", ref.Type(), ref))
		}
		return out
	}
	if got, want := describe(first), []string{"1:topic", "0:v1"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("refs of first commit: got %v, want %v", got, want)
	}
	if got, want := describe(second), []string{"2:main", "0:v2"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("refs of second commit: got %v, want %v", got, want)
	}
}

func TestBranches(t *testing.T) {
	upstream := newRepo(t)
	commit(t, upstream, "base", map[string]string{"a": "1"})
	gitRun(t, upstream, "branch", "doomed")
	dir := filepath.Join(t.TempDir(), "clone")
	gitRun(t, upstream, "clone", "--quiet", "file://"+upstream, dir)
	gitRun(t, dir, "branch", "--track", "doomed", "origin/doomed")
	gitRun(t, dir, "branch", "local")

	commit(t, upstream, "remote work", map[string]string{"b": "2"})
	gitRun(t, upstream, "branch", "--delete", "doomed")
	gitRun(t, dir, "fetch", "--quiet", "--prune")
	commit(t, dir, "local work 1", map[string]string{"c": "3"})
	commit(t, dir, "local work 2", map[string]string{"d": "4"})

	branches, err := open(t, dir).Branches()
	if err != nil {
		t.Fatal(err)
	}
	byName := make(map[string]*Branch)
	for _, b := range branches {
		byName[b.Name] = b
	}

	main := byName["main"]
	if main == nil || !main.Head || main.Type() != RefTypeHEAD || main.IsRemote() {
		t.Fatalf("got main %+v", main)
	}
	if main.Upstream == nil || main.Upstream.Name != "origin/main" || main.Ahead != 2 || main.Behind != 1 {
		t.Fatalf("got main upstream %+v ahead %d behind %d", main.Upstream, main.Ahead, main.Behind)
	}
	if main.When.IsZero() {
		t.Fatal("main has no commit date")
	}
	if b := byName["doomed"]; b == nil || b.Upstream != nil {
		t.Fatalf("branch with a deleted upstream: got %+v", b)
	}
	if b := byName["local"]; b == nil || b.Upstream != nil || b.Head || b.Type() != RefTypeBranch {
		t.Fatalf("got local %+v", b)
	}
	if b := byName["origin/main"]; b == nil || !b.IsRemote() {
		t.Fatalf("got origin/main %+v", b)
	}
}

func TestLoadStatus(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "base", map[string]string{"mod": "1\n", "both": "1\n", "del": "1\n", "conflict": "base\n"})

	gitRun(t, dir, "switch", "--quiet", "-c", "other")
	commit(t, dir, "other", map[string]string{"conflict": "other\n"})
	gitRun(t, dir, "switch", "--quiet", "main")
	commit(t, dir, "main", map[string]string{"conflict": "main\n"})
	cmd := exec.Command("git", "merge", "--quiet", "other")
	cmd.Dir = dir
	_ = cmd.Run() // conflicts

	writeFile(t, dir, "mod", "2\n")
	writeFile(t, dir, "both", "2\n")
	gitRun(t, dir, "add", "both")
	writeFile(t, dir, "both", "3\n")
	gitRun(t, dir, "rm", "--quiet", "del")
	writeFile(t, dir, "new dir/untracked", "x")
	writeFile(t, dir, "staged new", "x")
	gitRun(t, dir, "add", "staged new")

	st, err := open(t, dir).LoadStatus()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range st.Entities {
		got = append(got, fmt.Sprintf("%s staged=%v %s", e, e.Indexed(), e.StatusEntryString()))
	}
	// in the order of "git status": changes, conflicts, untracked files
	want := []string{
		"both staged=true Modified",
		"both staged=false Modified",
		"del staged=true Deleted",
		"mod staged=false Modified",
		"staged new staged=true Added",
		"conflict staged=false Conflicted",
		"new dir/ staged=false Untracked",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got entries\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if st.Branch.Name != "main" || st.Branch.Detached || st.Branch.Upstream != nil || len(st.Branch.Hash) != 40 {
		t.Fatalf("got branch %+v", st.Branch)
	}
}

func TestLoadStatusBranch(t *testing.T) {
	upstream := newRepo(t)
	commit(t, upstream, "base", map[string]string{"a": "1"})
	dir := filepath.Join(t.TempDir(), "clone")
	gitRun(t, upstream, "clone", "--quiet", "file://"+upstream, dir)
	commit(t, dir, "ahead", map[string]string{"a": "2"})

	st, err := open(t, dir).LoadStatus()
	if err != nil {
		t.Fatal(err)
	}
	b := st.Branch
	if b.Name != "main" || b.Upstream == nil || b.Upstream.Name != "origin/main" || b.Ahead != 1 || b.Behind != 0 {
		t.Fatalf("got branch %+v upstream %+v", b, b.Upstream)
	}

	gitRun(t, dir, "switch", "--quiet", "--detach", "HEAD~1")
	if st, err = open(t, dir).LoadStatus(); err != nil {
		t.Fatal(err)
	}
	if !st.Branch.Detached || st.Branch.Name != "" || st.Branch.Upstream != nil {
		t.Fatalf("got detached branch %+v", st.Branch)
	}
}
