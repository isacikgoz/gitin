package e2e

import (
	"strings"
	"testing"

	"github.com/isacikgoz/gitin/internal/gittest"
)

// pushScreen starts the push prompt, the title line alone since the
// choices contain "Push" as well
const pushScreen = "\nPush\n"

const checksConfig = `push:
  checks:
    - name: Lint
      run: echo LINT RAN
    - name: Tests
      run: |
        echo TESTS RAN
        exit $TESTS_EXIT
`

// pushRepo returns a remote and a clone with the checks and one commit to push
func pushRepo(t *testing.T) (remote, clone string) {
	t.Helper()
	remote, clone = gittest.NewRemote(t)
	gittest.Commit(t, clone, "Add the checks", map[string]string{".gitin.yml": checksConfig})
	return remote, clone
}

func pushed(t *testing.T, remote, clone string) bool {
	t.Helper()
	return gittest.Git(t, remote, "rev-parse", "main") == gittest.Git(t, clone, "rev-parse", "HEAD")
}

func TestPushRunsChecks(t *testing.T) {
	remote, clone := pushRepo(t)
	s := start(t, clone, []string{"push"}, "TESTS_EXIT=0")
	frame := s.waitFrame(pushScreen, 0, "> Run checks, then push", "Push without checks",
		"main → origin/main, 1 commit:", "] Add the checks", "Checks of .gitin.yml: Lint, Tests")
	if strings.Contains(frame, "Cancel") {
		t.Fatalf("offers Cancel, q cancels:\n%s", frame)
	}

	s.send(enter)
	if code := s.wait(); code != 0 {
		t.Fatalf("got exit code %d", code)
	}
	out := s.text()
	for _, want := range []string{"▶ Lint", "LINT RAN", "✔ Lint passed in", "TESTS RAN", "✔ Tests passed in", "▶ Pushing main to origin/main", "main -> main"} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not show %q", want)
		}
	}
	if !pushed(t, remote, clone) {
		t.Fatal("nothing was pushed")
	}
}

func TestPushWithoutChecks(t *testing.T) {
	remote, clone := pushRepo(t)
	s := start(t, clone, []string{"push"}, "TESTS_EXIT=0")
	s.waitFrame(pushScreen, 0, "> Run checks, then push")
	s.send(down, enter)
	if code := s.wait(); code != 0 {
		t.Fatalf("got exit code %d", code)
	}
	if strings.Contains(s.text(), "LINT RAN") {
		t.Fatal("checks ran")
	}
	if !pushed(t, remote, clone) {
		t.Fatal("nothing was pushed")
	}
}

func TestPushCancel(t *testing.T) {
	for name, keys := range map[string][]string{
		"q":      {"q"},
		"ctrl+c": {ctrlC},
	} {
		t.Run(name, func(t *testing.T) {
			remote, clone := pushRepo(t)
			s := start(t, clone, []string{"push"}, "TESTS_EXIT=0")
			s.waitFrame(pushScreen, 0, "> Run checks, then push")
			s.send(keys...)
			if code := s.wait(); code != 0 {
				t.Fatalf("got exit code %d", code)
			}
			if !strings.Contains(s.text(), "Push cancelled.") || strings.Contains(s.text(), "LINT RAN") {
				t.Fatalf("got output %q", s.text())
			}
			if pushed(t, remote, clone) {
				t.Fatal("pushed although cancelled")
			}
		})
	}
}

func TestPushFailingCheck(t *testing.T) {
	t.Run("cancel", func(t *testing.T) {
		remote, clone := pushRepo(t)
		s := start(t, clone, []string{"push"}, "TESTS_EXIT=3")
		s.waitFrame(pushScreen, 0, "> Run checks, then push")
		m := s.mark()
		s.send(enter)
		s.waitFrame(pushScreen, m, "> Cancel", "Push anyway", "Tests failed (exit status 3), nothing was pushed yet.")
		s.send(enter)
		if code := s.wait(); code != 1 {
			t.Fatalf("got exit code %d, want 1", code)
		}
		if !strings.Contains(s.text(), "✘ Tests failed after") || !strings.Contains(s.text(), "Tests failed, nothing was pushed") {
			t.Fatalf("got output %q", s.text())
		}
		if pushed(t, remote, clone) {
			t.Fatal("pushed although a check failed")
		}
	})

	t.Run("push anyway", func(t *testing.T) {
		remote, clone := pushRepo(t)
		s := start(t, clone, []string{"push"}, "TESTS_EXIT=3")
		s.waitFrame(pushScreen, 0, "> Run checks, then push")
		m := s.mark()
		s.send(enter)
		s.waitFrame(pushScreen, m, "> Cancel")
		s.send(down, enter)
		if code := s.wait(); code != 0 {
			t.Fatalf("got exit code %d", code)
		}
		if !pushed(t, remote, clone) {
			t.Fatal("nothing was pushed")
		}
	})
}

func TestPushStopCheck(t *testing.T) {
	remote, clone := gittest.NewRemote(t)
	gittest.Commit(t, clone, "Slow check", map[string]string{".gitin.yml": "push:\n  checks:\n    - name: Slow\n      run: printf 'CHECK %s\\n' STARTED; sleep 30\n"})
	s := start(t, clone, []string{"push"})
	s.waitFrame(pushScreen, 0, "> Run checks, then push")
	m := s.mark()
	s.send(enter)
	s.waitText(m, "CHECK STARTED")
	s.send(ctrlC)
	if code := s.wait(); code != 1 {
		t.Fatalf("got exit code %d, want 1", code)
	}
	if !strings.Contains(s.text(), "✘ Slow stopped after") || !strings.Contains(s.text(), "push cancelled") {
		t.Fatalf("got output %q", s.text())
	}
	if pushed(t, remote, clone) {
		t.Fatal("pushed although the check was stopped")
	}
}

// Checks get the keyboard once the prompt is closed, nothing reads it anymore.
func TestPushCheckReadsInput(t *testing.T) {
	remote, clone := gittest.NewRemote(t)
	gittest.Commit(t, clone, "Asking check", map[string]string{".gitin.yml": "push:\n  checks:\n    - run: |\n        echo 'Continue? '\n        read answer\n        echo \"ANSWER=$answer\"\n        test \"$answer\" = yes\n"})
	s := start(t, clone, []string{"push"})
	s.waitFrame(pushScreen, 0, "> Run checks, then push")
	m := s.mark()
	s.send(enter)
	s.waitText(m, "Continue?")
	s.send("yes\r")
	if code := s.wait(); code != 0 {
		t.Fatalf("got exit code %d", code)
	}
	if !strings.Contains(s.text(), "ANSWER=yes") {
		t.Fatalf("the check did not get the whole answer: %q", s.text())
	}
	if !pushed(t, remote, clone) {
		t.Fatal("nothing was pushed")
	}
}

func TestPushWithoutConfiguration(t *testing.T) {
	remote, clone := gittest.NewRemote(t)
	gittest.Commit(t, clone, "change", nil)
	s := start(t, clone, []string{"push"})
	frame := s.waitFrame(pushScreen, 0, "> Push", "No checks, add them to .git/gitin/config.yml to run them before pushing.")
	if strings.Contains(frame, "Run checks") || strings.Contains(frame, "Cancel") {
		t.Fatalf("offers checks without any:\n%s", frame)
	}
	s.send(enter)
	if code := s.wait(); code != 0 {
		t.Fatalf("got exit code %d", code)
	}
	if !pushed(t, remote, clone) {
		t.Fatal("nothing was pushed")
	}
}

func TestPushNewBranch(t *testing.T) {
	remote, clone := gittest.NewRemote(t)
	gittest.Git(t, clone, "checkout", "--quiet", "-b", "topic")
	gittest.Commit(t, clone, "topic work", nil)
	s := start(t, clone, []string{"push"})
	s.waitFrame(pushScreen, 0, "New branch topic is pushed to origin/topic with 1 commit:", "] topic work")
	s.send(enter)
	if code := s.wait(); code != 0 {
		t.Fatalf("got exit code %d", code)
	}
	if gittest.Git(t, remote, "rev-parse", "topic") != gittest.Git(t, clone, "rev-parse", "HEAD") {
		t.Fatal("topic was not pushed")
	}
	if got := gittest.Git(t, clone, "rev-parse", "--abbrev-ref", "topic@{upstream}"); got != "origin/topic" {
		t.Fatalf("got upstream %q", got)
	}
}

func TestPushRejected(t *testing.T) {
	remote, clone := gittest.NewRemote(t)
	other := gittest.Clone(t, remote)
	gittest.Commit(t, other, "theirs", map[string]string{"theirs": "1"})
	gittest.Git(t, other, "push", "--quiet")
	gittest.Commit(t, clone, "mine", map[string]string{"mine": "1"})
	gittest.Git(t, clone, "fetch", "--quiet")

	s := start(t, clone, []string{"push"})
	s.waitFrame(pushScreen, 0, "origin/main has 1 commit main doesn't have, the push is rejected until you pull it.")
	s.send(enter)
	if code := s.wait(); code != 1 {
		t.Fatalf("got exit code %d, want 1", code)
	}
	if !strings.Contains(s.text(), "rejected") || !strings.Contains(s.text(), "git push failed") {
		t.Fatalf("got output %q", s.text())
	}
}

func TestPushNothingToPush(t *testing.T) {
	_, clone := gittest.NewRemote(t)
	out, code := runGitin(t, clone, "push")
	if code != 0 || strings.TrimSpace(out) != "Nothing to push, main is up to date with origin/main." {
		t.Fatalf("got exit code %d and %q", code, out)
	}
}

func TestPushErrors(t *testing.T) {
	t.Run("detached HEAD", func(t *testing.T) {
		_, clone := gittest.NewRemote(t)
		gittest.Git(t, clone, "checkout", "--quiet", "--detach")
		out, code := runGitin(t, clone, "push")
		if code != 1 || !strings.Contains(out, "HEAD is detached") {
			t.Fatalf("got exit code %d and %q", code, out)
		}
	})

	t.Run("invalid configuration", func(t *testing.T) {
		_, clone := gittest.NewRemote(t)
		gittest.Commit(t, clone, "typo", map[string]string{".gitin.yml": "push:\n  check:\n    - run: make\n"})
		out, code := runGitin(t, clone, "push")
		if code != 1 || !strings.Contains(out, ".gitin.yml: yaml: unmarshal errors:") || !strings.Contains(out, "line 2: field check not found") {
			t.Fatalf("got exit code %d and %q", code, out)
		}
	})
}

func TestPushFromSubdirectory(t *testing.T) {
	remote, clone := gittest.NewRemote(t)
	gittest.Commit(t, clone, "checks", map[string]string{
		".gitin.yml": "push:\n  checks:\n    - name: In the root\n      run: test -f .gitin.yml\n",
		"sub/file":   "x",
	})
	s := start(t, clone+"/sub", []string{"push"})
	s.waitFrame(pushScreen, 0, "> Run checks, then push", "Checks of .gitin.yml: In the root")
	s.send(enter)
	if code := s.wait(); code != 0 {
		t.Fatalf("got exit code %d: %s", code, s.text())
	}
	if !pushed(t, remote, clone) {
		t.Fatal("nothing was pushed")
	}
}
