package cli

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/fatih/color"
	"github.com/isacikgoz/gitin/git"
	"github.com/isacikgoz/gitin/internal/gittest"
)

func TestRenderItem(t *testing.T) {
	dir := gittest.NewRepo(t)
	gittest.Commit(t, dir, "Speed up by 50%", map[string]string{"tracked": "1"})
	gittest.WriteFile(t, dir, "tracked", "2")
	gittest.Git(t, dir, "add", "tracked")
	gittest.WriteFile(t, dir, "untracked", "x")
	r := open(t, dir)
	st, err := r.LoadStatus()
	if err != nil {
		t.Fatal(err)
	}
	c := commits(t, r)[0]
	diff, err := c.Diff()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		item     interface{}
		selected bool
		want     string
		nameAttr color.Attribute
	}{
		{st.Entities[0], true, "> [M] tracked", color.FgGreen},
		{st.Entities[1], false, "  [U] untracked", color.FgRed},
		{c, false, "  [" + c.Hash[:7] + "] Speed up by 50%", color.FgWhite},
		{diff.Deltas()[0], false, "  [A] tracked", color.FgWhite},
		{&git.Branch{Name: "main", Head: true}, false, "  main *", color.FgGreen},
		{"something else", false, "  something else", color.FgWhite},
	}
	for _, tt := range tests {
		grid := renderItem(tt.item, nil, tt.selected)
		line := grid[0]
		if got := text(line); got != tt.want {
			t.Errorf("got %q, want %q", got, tt.want)
			continue
		}
		if last := line[len(line)-1]; !hasAttr(last, tt.nameAttr) {
			t.Errorf("%q: name has attributes %v, want %v", tt.want, last.Attr, tt.nameAttr)
		}
	}
}

func TestRenderRemoteBranch(t *testing.T) {
	branches, err := open(t, gittest.Clone(t, func() string {
		dir := gittest.NewRepo(t)
		gittest.Commit(t, dir, "base", nil)
		return dir
	}())).Branches()
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range branches {
		line := renderItem(b, nil, false)[0]
		want := color.FgWhite
		if b.IsRemote() {
			want = color.FgRed
		} else if b.Head {
			want = color.FgGreen
		}
		if !hasAttr(line[len(line)-1], want) {
			t.Errorf("%s: got attributes %v, want %v", b, line[len(line)-1].Attr, want)
		}
	}
}

func TestHighLightedText(t *testing.T) {
	// matches are byte offsets, underlined characters are marked with _
	mark := func(matches []int, s string) string {
		var out strings.Builder
		for _, c := range highLightedText(matches, color.FgWhite, s) {
			out.WriteRune(c.Ch)
			if hasAttr(c, color.Underline) {
				out.WriteRune('_')
			}
		}
		return out.String()
	}
	tests := []struct {
		matches []int
		in      string
		want    string
	}{
		{nil, "plain", "plain"},
		{[]int{0, 2}, "abc", "a_bc_"},
		{[]int{len("Ünï "), len("Ünï a")}, "Ünï ab", "Ünï a_b_"},
		{[]int{100}, "short", "short"},
	}
	for _, tt := range tests {
		if got := mark(tt.matches, tt.in); got != tt.want {
			t.Errorf("%q %v: got %q, want %q", tt.in, tt.matches, got, tt.want)
		}
	}
}

func TestBranchInfo(t *testing.T) {
	upstream := &git.Branch{Name: "origin/main"}
	tests := []struct {
		name   string
		branch *git.Branch
		yours  bool
		want   []string
	}{
		{"unknown", nil, true, []string{"Unable to load branch info"}},
		{"detached", &git.Branch{Detached: true, Hash: "0123456789abcdef"}, true, []string{"HEAD detached at 0123456"}},
		{"not tracking", &git.Branch{Name: "main"}, true, []string{"On branch main", "Your branch is not tracking a remote branch."}},
		{"someone else's", &git.Branch{Name: "topic"}, false, []string{"This branch is not tracking a remote branch."}},
		{"up to date", &git.Branch{Name: "main", Upstream: upstream}, true,
			[]string{"On branch main", "Your branch is up to date with origin/main."}},
		{"ahead", &git.Branch{Name: "main", Upstream: upstream, Ahead: 2}, true,
			[]string{"On branch main", "Your branch is ahead of origin/main by 2 commit(s).", "(\"push\" to publish your local commit(s))"}},
		{"behind", &git.Branch{Name: "main", Upstream: upstream, Behind: 3}, true,
			[]string{"On branch main", "Your branch is behind origin/main by 3 commit(s).", "(\"pull\" to update your local branch)"}},
		{"diverged", &git.Branch{Name: "main", Upstream: upstream, Ahead: 1, Behind: 4}, false,
			[]string{"This branch and origin/main have diverged,", "and have 1 and 4 different commits each, respectively.",
				"(\"pull\" to merge the remote branch into yours)"}},
	}
	for _, tt := range tests {
		if got := lines(branchInfo(tt.branch, tt.yours)); fmt.Sprint(got) != fmt.Sprint(tt.want) {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
	got := lines(workingTreeClean(&git.Branch{Name: "main"}))
	if got[len(got)-1] != "Nothing to commit, working tree clean" {
		t.Errorf("got %q", got)
	}
}

func TestCommitRefs(t *testing.T) {
	dir := gittest.NewRepo(t)
	gittest.Commit(t, dir, "first", nil)
	gittest.Git(t, dir, "tag", "--annotate", "--message=release", "v1.0")
	gittest.Git(t, dir, "branch", "topic")
	gittest.Commit(t, dir, "second", nil)
	r := open(t, dir)
	refs, err := r.Refs()
	if err != nil {
		t.Fatal(err)
	}
	all := commits(t, r)
	want := map[string]string{
		"second": "(HEAD -> main)",
		"first":  "(topic, tag: v1.0)",
	}
	for _, c := range all {
		if got := text(commitRefs(refs, c)); got != want[c.Summary] {
			t.Errorf("%s: got %q, want %q", c.Summary, got, want[c.Summary])
		}
	}
	if got := commitRefs(map[string][]git.Ref{all[0].Hash: nil}, all[0]); len(got) != 0 {
		t.Errorf("got %q for a commit without refs", text(got))
	}
}

func TestLogInfo(t *testing.T) {
	dir := gittest.NewRepo(t)
	gittest.Commit(t, dir, "root", map[string]string{"one.txt": "a\n", "many.txt": "a\nb\nc\n"})
	gittest.WriteFile(t, dir, "bin.dat", "\x00\x01")
	gittest.Commit(t, dir, "change", map[string]string{"one.txt": "b\n", "many.txt": "a\n", "same.txt": ""})
	r := open(t, dir)
	refs, err := r.Refs()
	if err != nil {
		t.Fatal(err)
	}
	l := &log{repository: r, refs: refs}
	c := commits(t, r)[0]

	got := lines(l.logInfo(c))
	want := []string{"Author Ada Lovelace <ada@example.com>", "When   " + "less than a minute ago", "(HEAD -> main)"}
	if got[0] != want[0] || !strings.HasPrefix(got[1], "When   ") || got[2] != want[2] {
		t.Fatalf("got %q, want %q", got, want)
	}
	if c.Author.When.After(time.Now()) {
		t.Fatal("commit date is in the future")
	}

	diff, err := c.Diff()
	if err != nil {
		t.Fatal(err)
	}
	wantDelta := map[string]string{
		"bin.dat":  "Binary file.",
		"many.txt": "2 deletions.",
		"one.txt":  "1 addition, 1 deletion.",
		"same.txt": "",
	}
	for _, d := range diff.Deltas() {
		if got := strings.Join(lines(l.logInfo(d)), "\n"); got != wantDelta[d.String()] {
			t.Errorf("%s: got %q, want %q", d, got, wantDelta[d.String()])
		}
	}
	if len(l.logInfo(nil)) != 0 {
		t.Error("info for no item")
	}
}
