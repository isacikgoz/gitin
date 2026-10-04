package git

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/isacikgoz/gitin/internal/gittest"
)

func describeEntries(entries []*StatusEntry) string {
	var lines []string
	for _, e := range entries {
		lines = append(lines, fmt.Sprintf("%s staged=%v %s", e, e.Indexed(), e.StatusEntryString()))
	}
	return strings.Join(lines, "\n")
}

func TestLoadStatus(t *testing.T) {
	dir := gittest.NewRepo(t)
	gittest.Commit(t, dir, "base", map[string]string{"mod": "1\n", "both": "1\n", "del": "1\n", "conflict": "base\n", "gone": "1\n"})

	gittest.Git(t, dir, "checkout", "--quiet", "-b", "other")
	gittest.Commit(t, dir, "other", map[string]string{"conflict": "other\n"})
	gittest.Git(t, dir, "checkout", "--quiet", "main")
	gittest.Commit(t, dir, "main", map[string]string{"conflict": "main\n"})
	cmd := exec.Command("git", "merge", "--quiet", "other")
	cmd.Dir = dir
	_ = cmd.Run() // conflicts

	gittest.WriteFile(t, dir, "mod", "2\n")
	gittest.WriteFile(t, dir, "both", "2\n")
	gittest.Git(t, dir, "add", "both")
	gittest.WriteFile(t, dir, "both", "3\n")
	gittest.Git(t, dir, "rm", "--quiet", "del")
	gittest.Git(t, dir, "rm", "--quiet", "--cached", "gone")
	gittest.WriteFile(t, dir, "new dir/untracked", "x")
	gittest.WriteFile(t, dir, "staged new", "x")
	gittest.Git(t, dir, "add", "staged new")
	gittest.WriteFile(t, dir, "ignored.log", "x")
	gittest.WriteFile(t, dir, ".git/info/exclude", "*.log\n")

	st, err := open(t, dir).LoadStatus()
	if err != nil {
		t.Fatal(err)
	}
	// in the order of "git status": changes, conflicts, untracked files
	want := strings.Join([]string{
		"both staged=true Modified",
		"both staged=false Modified",
		"del staged=true Deleted",
		"gone staged=true Deleted",
		"mod staged=false Modified",
		"staged new staged=true Added",
		"conflict staged=false Conflicted",
		"gone staged=false Untracked",
		"new dir/ staged=false Untracked",
	}, "\n")
	if got := describeEntries(st.Entities); got != want {
		t.Fatalf("got entries\n%s\nwant\n%s", got, want)
	}
	if st.Branch.Name != "main" || st.Branch.Detached || st.Branch.Upstream != nil || len(st.Branch.Hash) != 40 {
		t.Fatalf("got branch %+v", st.Branch)
	}
}

func TestLoadStatusWithoutCommits(t *testing.T) {
	dir := gittest.NewRepo(t)
	gittest.WriteFile(t, dir, "a", "1")
	gittest.Git(t, dir, "add", "a")
	st, err := open(t, dir).LoadStatus()
	if err != nil {
		t.Fatal(err)
	}
	if got := describeEntries(st.Entities); got != "a staged=true Added" {
		t.Fatalf("got entries %s", got)
	}
	if st.Branch.Name != "main" || st.Branch.Hash != "" {
		t.Fatalf("got branch %+v", st.Branch)
	}
}

func TestLoadStatusBranch(t *testing.T) {
	upstream := gittest.NewRepo(t)
	gittest.Commit(t, upstream, "base", map[string]string{"a": "1"})
	dir := gittest.Clone(t, upstream)
	gittest.Commit(t, dir, "ahead", map[string]string{"a": "2"})

	st, err := open(t, dir).LoadStatus()
	if err != nil {
		t.Fatal(err)
	}
	b := st.Branch
	if b.Name != "main" || b.FullName != "refs/heads/main" || !b.Head || b.Type() != RefTypeHEAD {
		t.Fatalf("got branch %+v", b)
	}
	if b.Upstream == nil || b.Upstream.Name != "origin/main" || b.Ahead != 1 || b.Behind != 0 {
		t.Fatalf("got upstream %+v ahead %d behind %d", b.Upstream, b.Ahead, b.Behind)
	}

	gittest.Git(t, dir, "checkout", "--quiet", "--detach", "HEAD~1")
	if st, err = open(t, dir).LoadStatus(); err != nil {
		t.Fatal(err)
	}
	if !st.Branch.Detached || st.Branch.Name != "" || st.Branch.Upstream != nil {
		t.Fatalf("got detached branch %+v", st.Branch)
	}
}

func TestParseStatus(t *testing.T) {
	const ordinary = "N... 100644 100644 100644 1111111111111111111111111111111111111111 2222222222222222222222222222222222222222"
	out := strings.Join([]string{
		"# branch.oid (initial)",
		"# branch.head topic",
		"# branch.upstream origin/topic",
		"# branch.ab +0 -4",
		"1 .T " + ordinary + " link",
		"1 .A " + ordinary + " intent to add",
		"2 RM " + ordinary + " R100 new name", "old name",
		"! ignored",
		"",
	}, "\x00")
	st, err := parseStatus([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"link staged=false Type change",
		"intent to add staged=false Added",
		"new name staged=true Renamed",
		"new name staged=false Modified",
	}, "\n")
	if got := describeEntries(st.Entities); got != want {
		t.Fatalf("got entries\n%s\nwant\n%s", got, want)
	}
	b := st.Branch
	if b.Hash != "" || b.Name != "topic" || b.Upstream.Name != "origin/topic" || b.Ahead != 0 || b.Behind != 4 {
		t.Fatalf("got branch %+v", b)
	}

	if _, err := parseStatus([]byte("1 M short\x00")); err == nil {
		t.Fatal("malformed record: no error")
	}
}

func TestStatusEntryString(t *testing.T) {
	for entryType, want := range map[StatusEntryType]string{
		StatusEntryTypeNew: "Added", StatusEntryTypeModified: "Modified", StatusEntryTypeDeleted: "Deleted",
		StatusEntryTypeRenamed: "Renamed", StatusEntryTypeUntracked: "Untracked",
		StatusEntryTypeTypeChange: "Type change", StatusEntryTypeConflicted: "Conflicted", StatusEntryType(99): "Unknown",
	} {
		if got := (&StatusEntry{EntryType: entryType}).StatusEntryString(); got != want {
			t.Errorf("%d: got %q, want %q", entryType, got, want)
		}
	}
}
