package git

import (
	"fmt"
	"testing"

	"github.com/isacikgoz/gitin/internal/gittest"
)

func TestRefs(t *testing.T) {
	dir := gittest.NewRepo(t)
	first := gittest.Commit(t, dir, "first", nil)
	gittest.Git(t, dir, "tag", "--annotate", "--message=release", "v1")
	second := gittest.Commit(t, dir, "second", nil)
	gittest.Git(t, dir, "tag", "v2")
	gittest.Git(t, dir, "branch", "topic", first)
	clone := gittest.Clone(t, dir)

	refs, err := open(t, clone).Refs()
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
	want := map[string][]string{
		first:  {"1:origin/topic", "0:v1"},
		second: {"2:main", "1:origin/HEAD", "1:origin/main", "0:v2"},
	}
	for hash, want := range want {
		if got := describe(hash); fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("refs of %s: got %v, want %v", hash, got, want)
		}
	}
	for _, ref := range refs[second] {
		if b, ok := ref.(*Branch); ok && b.Name == "origin/main" && !b.IsRemote() {
			t.Error("origin/main is not a remote branch")
		}
	}
}

func TestRefsOfEmptyRepository(t *testing.T) {
	refs, err := open(t, gittest.NewRepo(t)).Refs()
	if err != nil || len(refs) != 0 {
		t.Fatalf("got %v, %v", refs, err)
	}
}

func TestBranches(t *testing.T) {
	upstream := gittest.NewRepo(t)
	gittest.Commit(t, upstream, "base", map[string]string{"a": "1"})
	gittest.Git(t, upstream, "branch", "doomed")
	dir := gittest.Clone(t, upstream)
	gittest.Git(t, dir, "branch", "--track", "doomed", "origin/doomed")
	gittest.Git(t, dir, "branch", "local")

	gittest.Commit(t, upstream, "remote work", map[string]string{"b": "2"})
	gittest.Git(t, upstream, "branch", "--delete", "doomed")
	gittest.Git(t, dir, "fetch", "--quiet", "--prune")
	gittest.Commit(t, dir, "local work 1", map[string]string{"c": "3"})
	gittest.Commit(t, dir, "local work 2", map[string]string{"d": "4"})

	branches, err := open(t, dir).Branches()
	if err != nil {
		t.Fatal(err)
	}
	byName := make(map[string]*Branch)
	var names []string
	for _, b := range branches {
		byName[b.Name] = b
		names = append(names, b.String())
	}
	if got, want := fmt.Sprint(names), "[doomed local main origin/HEAD origin/main]"; got != want {
		t.Fatalf("got branches %s, want %s", got, want)
	}

	main := byName["main"]
	if !main.Head || main.Type() != RefTypeHEAD || main.IsRemote() || main.FullName != "refs/heads/main" {
		t.Fatalf("got main %+v", main)
	}
	if main.Upstream == nil || main.Upstream.Name != "origin/main" || main.Upstream.FullName != "refs/remotes/origin/main" ||
		main.Ahead != 2 || main.Behind != 1 {
		t.Fatalf("got main upstream %+v ahead %d behind %d", main.Upstream, main.Ahead, main.Behind)
	}
	if main.When.IsZero() || len(main.Hash) != 40 {
		t.Fatalf("got main %+v", main)
	}
	if b := byName["doomed"]; b.Upstream != nil {
		t.Fatalf("branch with a deleted upstream: got upstream %+v", b.Upstream)
	}
	if b := byName["local"]; b.Upstream != nil || b.Head || b.Type() != RefTypeBranch {
		t.Fatalf("got local %+v", b)
	}
	if b := byName["origin/main"]; !b.IsRemote() || b.Head || b.Upstream != nil {
		t.Fatalf("got origin/main %+v", b)
	}
}

func TestParseTrack(t *testing.T) {
	tests := map[string][2]int{
		"":                   {0, 0},
		"ahead 3":            {3, 0},
		"behind 2":           {0, 2},
		"ahead 1, behind 12": {1, 12},
		"gone":               {0, 0},
	}
	for track, want := range tests {
		ahead, behind := parseTrack(track)
		if [2]int{ahead, behind} != want {
			t.Errorf("%q: got %d %d, want %v", track, ahead, behind, want)
		}
	}
}
