package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/isacikgoz/gitin/config"
	"github.com/isacikgoz/gitin/git"
	"github.com/isacikgoz/gitin/internal/gittest"
)

func TestRunChecks(t *testing.T) {
	dir := t.TempDir()
	out := captureTerminal(t)
	checks := []config.Check{
		{Name: "first", Run: "echo first ran"},
		{Name: "where", Run: "pwd"},
		{Name: "Lint", Run: "echo lint problem >&2\nexit 4"},
		{Name: "last", Run: "echo last ran"},
	}
	failure, err := runChecks(dir, checks)
	if err != nil {
		t.Fatal(err)
	}
	if failure == nil || failure.check.Name != "Lint" {
		t.Fatalf("got failure %+v", failure)
	}
	var exitErr *exec.ExitError
	if !errors.As(failure.err, &exitErr) || exitErr.ExitCode() != 4 {
		t.Fatalf("got error %v, want exit status 4", failure.err)
	}

	got := out.String()
	for _, want := range []string{
		"▶ first  echo first ran\nfirst ran\n✔ first passed in ",
		"✔ where passed in ",
		"▶ Lint  echo lint problem >&2 …\nlint problem\n✘ Lint failed after ",
		": exit status 4\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output does not contain %q:\n%s", want, got)
		}
	}
	// checks run in the given directory and stop at the first failure
	if real, _ := filepath.EvalSymlinks(dir); !strings.Contains(got, real+"\n") && !strings.Contains(got, dir+"\n") {
		t.Errorf("check did not run in %s:\n%s", dir, got)
	}
	if strings.Contains(got, "last ran") {
		t.Errorf("a check ran after a failure:\n%s", got)
	}
}

func TestRunChecksAllPass(t *testing.T) {
	out := captureTerminal(t)
	failure, err := runChecks(t.TempDir(), []config.Check{{Name: "true", Run: "true"}})
	if failure != nil || err != nil {
		t.Fatalf("got failure %+v, error %v", failure, err)
	}
	// a check named after its command is not shown twice
	if got := out.String(); !strings.HasPrefix(got, "▶ true\n✔ true passed in ") {
		t.Fatalf("got %q", got)
	}
}

func TestPushInfo(t *testing.T) {
	commits := func(summaries ...string) []*git.Commit {
		var out []*git.Commit
		for _, s := range summaries {
			out = append(out, &git.Commit{Hash: "0123456789abcdef0123456789abcdef01234567", Summary: s})
		}
		return out
	}
	checks := &config.Config{Files: []string{".gitin.yml", ".git/gitin/config.yml"}, Push: config.Hook{Checks: []config.Check{{Name: "Lint"}, {Name: "Tests"}}}}

	tests := []struct {
		name   string
		target *git.PushTarget
		config *config.Config
		want   []string
	}{
		{
			"ahead",
			&git.PushTarget{Branch: "main", RemoteBranch: "origin/main", Ahead: 1, Commits: commits("Fix it")},
			checks,
			[]string{"main → origin/main, 1 commit:", "  [0123456] Fix it", "Checks of .gitin.yml and .git/gitin/config.yml: Lint, Tests"},
		},
		{
			"many commits",
			&git.PushTarget{Branch: "main", RemoteBranch: "origin/main", Ahead: 12, Commits: commits("1", "2", "3", "4", "5", "6", "7", "8", "9", "10")},
			&config.Config{Personal: ".git/gitin/config.yml"},
			[]string{"main → origin/main, 12 commits:", "  [0123456] 1", "  [0123456] 2", "  [0123456] 3", "  [0123456] 4", "  [0123456] 5",
				"  and 7 more", "No checks, add them to .git/gitin/config.yml to run them before pushing."},
		},
		{
			"new branch",
			&git.PushTarget{Branch: "topic", RemoteBranch: "origin/topic", SetUpstream: true, Ahead: 2, Commits: commits("b", "a")},
			checks,
			[]string{"New branch topic is pushed to origin/topic with 2 commits:", "  [0123456] b", "  [0123456] a", "Checks of .gitin.yml and .git/gitin/config.yml: Lint, Tests"},
		},
		{
			"diverged",
			&git.PushTarget{Branch: "main", RemoteBranch: "origin/main", Ahead: 1, Behind: 1, Commits: commits("mine")},
			checks,
			[]string{"main → origin/main, 1 commit:", "  [0123456] mine",
				"origin/main has 1 commit main doesn't have, the push is rejected until you pull it.", "Checks of .gitin.yml and .git/gitin/config.yml: Lint, Tests"},
		},
	}
	for _, tt := range tests {
		if got := lines(pushInfo(tt.target, tt.config)); strings.Join(got, "\n") != strings.Join(tt.want, "\n") {
			t.Errorf("%s: got\n%s\nwant\n%s", tt.name, strings.Join(got, "\n"), strings.Join(tt.want, "\n"))
		}
	}
	behind := &git.PushTarget{Branch: "main", RemoteBranch: "origin/main", Ahead: 1, Behind: 3}
	if got := lines(pushInfo(behind, checks)); !strings.Contains(strings.Join(got, "\n"), "has 3 commits main doesn't have, the push is rejected until you pull them.") {
		t.Errorf("got %q", got)
	}
}

func TestPushMessages(t *testing.T) {
	if got := nothingToPush(&git.PushTarget{Branch: "main", RemoteBranch: "origin/main"}); got != "Nothing to push, main is up to date with origin/main." {
		t.Errorf("got %q", got)
	}
	if got := nothingToPush(&git.PushTarget{Branch: "main", RemoteBranch: "origin/main", Behind: 2}); got != "Nothing to push, main is behind origin/main by 2 commits." {
		t.Errorf("got %q", got)
	}
	failure := &checkFailure{check: config.Check{Name: "Lint"}, err: errors.New("exit status 1")}
	if got := lines(failureInfo(failure, "nothing was pushed yet")); len(got) != 1 || got[0] != "Lint failed (exit status 1), nothing was pushed yet." {
		t.Errorf("got %q", got)
	}
	for d, want := range map[time.Duration]string{
		35*time.Millisecond + 400*time.Microsecond: "35ms",
		3240 * time.Millisecond:                    "3.2s",
		62*time.Second + 340*time.Millisecond:      "1m2.3s",
	} {
		if got := duration(d); got != want {
			t.Errorf("%v: got %q, want %q", d, got, want)
		}
	}
	for command, want := range map[string]string{
		"go test ./...":                  "go test ./...",
		"  make lint\n":                  "make lint",
		"set -e\ngo vet ./...\ngo test ": "set -e …",
	} {
		if got := firstLine(command); got != want {
			t.Errorf("%q: got %q, want %q", command, got, want)
		}
	}
}

func TestPushFailure(t *testing.T) {
	out := captureTerminal(t)
	_, clone := gittest.NewRemote(t)
	gittest.Commit(t, clone, "change", nil)
	gittest.Git(t, clone, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "gone.git"))
	r := open(t, clone)
	target, err := r.PushTarget()
	if err != nil {
		t.Fatal(err)
	}
	if err := push(r, target); err == nil || err.Error() != "git push failed" {
		t.Fatalf("got error %v", err)
	}
	if !strings.Contains(out.String(), "▶ Pushing main to origin/main") || !strings.Contains(out.String(), "gone.git") {
		t.Fatalf("got output %q", out.String())
	}
}

func TestRunChecksStopsOnInterrupt(t *testing.T) {
	out := captureTerminal(t)
	checks := []config.Check{
		// like Ctrl-C in the terminal, which signals gitin and the check, the
		// check waits for gitin to receive the signal
		{Name: "Interrupted", Run: "kill -INT $PPID; sleep 0.5"},
		{Name: "Next", Run: "echo NEXT RAN"},
	}
	failure, err := runChecks(t.TempDir(), checks)
	if !errors.Is(err, errStopped) || failure != nil {
		t.Fatalf("got failure %+v, error %v", failure, err)
	}
	if got := out.String(); !strings.Contains(got, "✘ Interrupted stopped after") || strings.Contains(got, "NEXT RAN") {
		t.Fatalf("got output %q", got)
	}
}

// Ctrl-C right before a check starts only reaches gitin, which passes it on
func TestRunChecksForwardsEarlyInterrupt(t *testing.T) {
	out := captureTerminal(t)
	beforeCheckStart = func() {
		_ = syscall.Kill(os.Getpid(), syscall.SIGINT)
		time.Sleep(100 * time.Millisecond) // the signal arrives
	}
	defer func() { beforeCheckStart = func() {} }()

	start := time.Now()
	failure, err := runChecks(t.TempDir(), []config.Check{{Name: "Slow", Run: "sleep 10"}})
	if !errors.Is(err, errStopped) || failure != nil {
		t.Fatalf("got failure %+v, error %v", failure, err)
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Fatalf("the check ran for %v, it did not get the interrupt", took)
	}
	if !strings.Contains(out.String(), "✘ Slow stopped after") {
		t.Fatalf("got output %q", out.String())
	}
}

// A check killed by Ctrl-C counts as stopped, also if gitin didn't see the signal yet
func TestRunChecksStoppedBySignal(t *testing.T) {
	out := captureTerminal(t)
	failure, err := runChecks(t.TempDir(), []config.Check{{Name: "Killed", Run: "kill -INT $$; sleep 5"}, {Name: "Next", Run: "echo NEXT RAN"}})
	if !errors.Is(err, errStopped) || failure != nil {
		t.Fatalf("got failure %+v, error %v", failure, err)
	}
	if got := out.String(); !strings.Contains(got, "✘ Killed stopped after") || strings.Contains(got, "NEXT RAN") {
		t.Fatalf("got output %q", got)
	}
	if interrupted(nil) || interrupted(errors.New("other")) {
		t.Fatal("errors without a signal count as interrupted")
	}
}
