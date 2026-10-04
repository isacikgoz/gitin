package git

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/isacikgoz/gitin/internal/gittest"
)

func collect(t *testing.T, r *Repository) ([]*Commit, error) {
	t.Helper()
	ch, wait := r.Commits(context.Background())
	var commits []*Commit
	for c := range ch {
		commits = append(commits, c)
	}
	return commits, wait()
}

// fastImport creates n commits on main quickly
func fastImport(t *testing.T, dir string, n int) {
	t.Helper()
	var script strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&script, "commit refs/heads/main\nmark :%d\ncommitter A <a@b> %d +0000\ndata 6\nc%05d", i, 1000000+i, i)
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
}

func TestCommits(t *testing.T) {
	dir := gittest.NewRepo(t)
	root := gittest.Commit(t, dir, "root commit", map[string]string{"a": "1\n"})
	second := gittest.Commit(t, dir, "Speed up by 50%", map[string]string{"b": "2\n"})
	third := gittest.Commit(t, dir, "third\n\nwith a body", map[string]string{"c": "3\n"})

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
	if commits[0].Summary != "third" {
		t.Fatalf("got summary %q, want only the subject", commits[0].Summary)
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

func TestCommitsUseMailmap(t *testing.T) {
	dir := gittest.NewRepo(t)
	gittest.Commit(t, dir, "first", map[string]string{".mailmap": "Augusta Ada King <ada@example.org> <ada@example.com>\n"})
	commits, err := collect(t, open(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	if a := commits[0].Author; a.Name != "Augusta Ada King" || a.Email != "ada@example.org" {
		t.Fatalf("got author %+v, want the one from .mailmap", a)
	}
}

func TestCommitsIgnoreLogConfig(t *testing.T) {
	dir := gittest.NewRepo(t)
	gittest.Commit(t, dir, "first", nil)
	// settings that change git log's output must not break parsing
	gittest.Git(t, dir, "config", "log.showSignature", "true")
	gittest.Git(t, dir, "config", "log.decorate", "full")
	gittest.Git(t, dir, "config", "format.pretty", "oneline")
	commits, err := collect(t, open(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 1 || commits[0].Summary != "first" {
		t.Fatalf("got commits %v", commits)
	}
}

func TestCommitsShallowClone(t *testing.T) {
	dir := gittest.NewRepo(t)
	for i := range 3 {
		gittest.Commit(t, dir, fmt.Sprintf("commit %d", i), map[string]string{"a": fmt.Sprint(i)})
	}
	clone := gittest.Clone(t, dir, "--depth=2")

	commits, err := collect(t, open(t, clone))
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 2 {
		t.Fatalf("got %d commits, want 2", len(commits))
	}
}

func TestCommitsWithoutCommits(t *testing.T) {
	_, err := collect(t, open(t, gittest.NewRepo(t)))
	if err == nil || !strings.Contains(err.Error(), "does not have any commits") {
		t.Fatalf("got error %v", err)
	}
}

func TestCommitsMany(t *testing.T) {
	dir := gittest.NewRepo(t)
	fastImport(t, dir, 5000)
	commits, err := collect(t, open(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 5000 || commits[0].Summary != "c05000" || commits[4999].Summary != "c00001" {
		t.Fatalf("got %d commits from %v to %v", len(commits), commits[0], commits[len(commits)-1])
	}
}

func TestCommitsCancel(t *testing.T) {
	dir := gittest.NewRepo(t)
	fastImport(t, dir, 5000)

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
	dir := gittest.NewRepo(t)
	gittest.Commit(t, dir, "root", map[string]string{"keep.txt": "a\nb\n", "gone.txt": "x\n", "50% off.txt": "y\n"})
	gittest.WriteFile(t, dir, "bin.dat", "\x00\x01\x02")
	gittest.Commit(t, dir, "second", map[string]string{"keep.txt": "a\nc\nd\n", "dir/new.txt": "n\n"})
	gittest.Git(t, dir, "rm", "--quiet", "gone.txt")
	gittest.Git(t, dir, "commit", "--quiet", "-m", "remove")

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
			"bin.dat":     {DeltaAdded, 0, 0, true},
			"dir/new.txt": {DeltaAdded, 1, 0, false},
			"keep.txt":    {DeltaModified, 2, 1, false},
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
			if d.OldFile.Path != d.NewFile.Path || len(d.NewFile.Hash) != 40 {
				t.Fatalf("delta %s has files %+v %+v", d, d.OldFile, d.NewFile)
			}
		}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("commit %q: got %v, want %v", c.Summary, got, want)
		}
	}
}

func TestCommitDiffOfEmptyCommit(t *testing.T) {
	dir := gittest.NewRepo(t)
	gittest.Commit(t, dir, "root", map[string]string{"a": "1"})
	gittest.Commit(t, dir, "empty", nil)
	commits, err := collect(t, open(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	diff, err := commits[0].Diff()
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.Deltas()) != 0 {
		t.Fatalf("got deltas %v", diff.Deltas())
	}
}

func TestCommitDiffOfMergeComparesFirstParent(t *testing.T) {
	dir := gittest.NewRepo(t)
	gittest.Commit(t, dir, "root", map[string]string{"a": "1\n"})
	gittest.Git(t, dir, "checkout", "--quiet", "-b", "topic")
	gittest.Commit(t, dir, "topic", map[string]string{"topic.txt": "t\n"})
	gittest.Git(t, dir, "checkout", "--quiet", "main")
	gittest.Commit(t, dir, "main", map[string]string{"main.txt": "m\n"})
	gittest.Git(t, dir, "merge", "--quiet", "--no-edit", "topic")

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

func TestParseDiffTree(t *testing.T) {
	const hashA = "1111111111111111111111111111111111111111"
	const hashB = "2222222222222222222222222222222222222222"
	raw := func(status, path string) string {
		return ":100644 100644 " + hashA + " " + hashB + " " + status + "\x00" + path + "\x00"
	}

	deltas, err := parseDiffTree([]byte(raw("M", "a b") + raw("T", "link") + raw("X", "odd") + "1\t2\ta b\x00-\t-\tlink\x000\t0\todd\x00"))
	if err != nil {
		t.Fatal(err)
	}
	got := fmt.Sprintf("%v %d %d %v | %v %v | %v", deltas[0].Status, deltas[0].Additions, deltas[0].Deletions, deltas[0].Binary,
		deltas[1].Status, deltas[1].Binary, deltas[2].Status)
	want := fmt.Sprintf("%v 1 2 false | %v true | %v", DeltaModified, DeltaTypeChange, DeltaUnreadable)
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}

	if deltas, err := parseDiffTree(nil); err != nil || len(deltas) != 0 {
		t.Fatalf("empty output: got %v, %v", deltas, err)
	}

	for name, out := range map[string]string{
		"malformed raw record":        ":100644 M\x00a\x001\t1\ta\x00",
		"missing line counts":         raw("M", "a"),
		"line counts of another file": raw("M", "a") + "1\t1\tb\x00",
		"truncated line counts":       raw("M", "a") + "1\x00",
	} {
		if _, err := parseDiffTree([]byte(out)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestDeltaStatusString(t *testing.T) {
	codes := map[byte]string{
		'A': "Added", 'D': "Deleted", 'M': "Modified", 'R': "Renamed", 'C': "Copied",
		'T': "TypeChange", 'U': "Conflicted", 'X': "Unreadable",
	}
	for code, want := range codes {
		d := &DiffDelta{Status: deltaStatusFromCode(code)}
		if got := d.DeltaStatusString(); got != want {
			t.Errorf("%c: got %q, want %q", code, got, want)
		}
	}
	for status, want := range map[DeltaStatus]string{
		DeltaUnmodified: "Unmodified", DeltaIgnored: "Ignored", DeltaUntracked: "Untracked", DeltaStatus(99): " ",
	} {
		if got := (&DiffDelta{Status: status}).DeltaStatusString(); got != want {
			t.Errorf("%d: got %q, want %q", status, got, want)
		}
	}
}
