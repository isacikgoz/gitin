package e2e

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/isacikgoz/gitin/internal/gittest"
)

// personal checks are in the git directory, nothing to commit or ignore
func TestPersonalChecks(t *testing.T) {
	remote, clone := gittest.NewRemote(t)
	gittest.WriteFile(t, clone, ".git/gitin/config.yml", "push:\n  checks:\n    - name: Mine\n      run: printf 'MINE %s\\n' RAN\n")
	gittest.Commit(t, clone, "change", nil)
	if got := shortStatus(t, clone); got != "" {
		t.Fatalf("the personal configuration shows up in the status: %q", got)
	}

	s := start(t, clone, []string{"push"})
	s.waitFrame(pushScreen, 0, "> Run checks, then push", "Checks of .git/gitin/config.yml: Mine")
	s.send(enter)
	if code := s.wait(); code != 0 {
		t.Fatalf("got exit code %d", code)
	}
	if !strings.Contains(s.text(), "MINE RAN") || !pushed(t, remote, clone) {
		t.Fatalf("got output %q", s.text())
	}
}

// the team's checks run first, then the personal ones
func TestSharedAndPersonalChecks(t *testing.T) {
	_, clone := gittest.NewRemote(t)
	gittest.Commit(t, clone, "team checks", map[string]string{".gitin.yml": "push:\n  checks:\n    - name: Team\n      run: printf 'TEAM %s\\n' RAN\n"})
	gittest.WriteFile(t, clone, ".git/gitin/config.yml", "push:\n  checks:\n    - name: Mine\n      run: printf 'MINE %s\\n' RAN\n")

	s := start(t, clone, []string{"push"})
	s.waitFrame(pushScreen, 0, "Checks of .gitin.yml and .git/gitin/config.yml: Team, Mine")
	s.send(enter)
	if code := s.wait(); code != 0 {
		t.Fatalf("got exit code %d", code)
	}
	out := s.text()
	if team, mine := strings.Index(out, "TEAM RAN"), strings.Index(out, "MINE RAN"); team < 0 || mine < team {
		t.Fatalf("got output %q", out)
	}
}

// all worktrees of a clone use its personal checks
func TestPersonalChecksInWorktree(t *testing.T) {
	_, clone := gittest.NewRemote(t)
	gittest.WriteFile(t, clone, ".git/gitin/config.yml", "commit:\n  checks:\n    - name: Mine\n      run: printf 'MINE %s\\n' RAN\n")
	linked := filepath.Join(t.TempDir(), "linked")
	gittest.Git(t, clone, "worktree", "add", "-b", "topic", linked)
	gittest.WriteFile(t, linked, "new.txt", "new\n")
	gittest.Git(t, linked, "add", "new.txt")

	s := start(t, linked, []string{"status"}, editor(t, "from the worktree"))
	s.waitFrame("Files", 0, "new.txt")
	m := s.mark()
	s.send("c")
	s.waitFrame(commitScreen, m, "> Run checks, then commit", "Checks of ")
	m = s.mark()
	s.send(enter)
	if code := s.wait(); code != 0 {
		t.Fatalf("got exit code %d", code)
	}
	if !strings.Contains(s.text()[m:], "MINE RAN") || lastCommit(t, linked) != "from the worktree" {
		t.Fatalf("got output %q", s.text()[m:])
	}
}
