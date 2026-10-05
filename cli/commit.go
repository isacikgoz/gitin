package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/isacikgoz/gitin/config"
	"github.com/isacikgoz/gitin/git"
	"github.com/isacikgoz/gitin/term"
)

// commitMode is a new commit or an amend of the last one
type commitMode struct {
	arg  string // argument of git commit
	verb string
	// outcome says what didn't happen when the user cancels
	outcome string
}

var (
	newCommit   = commitMode{arg: "--edit", verb: "commit", outcome: "nothing was committed"}
	amendCommit = commitMode{arg: "--amend", verb: "amend", outcome: "the commit was not amended"}
)

// checkAndCommit commits, after the commit checks of the configuration if
// the user wants to run them
func (s *status) checkAndCommit(m commitMode) error {
	// a merge can be committed without staged changes
	if m == newCommit && !s.hasStaged() && !s.repository.Merging() {
		return errors.New("nothing to commit, stage changes with space or a first")
	}
	cfg, err := config.Load(s.repository.Path())
	if err != nil {
		return err
	}
	if len(cfg.Commit.Checks) == 0 {
		return s.bareCommit(m.arg)
	}
	return s.prompt.Suspend(func() error {
		title := strings.ToUpper(m.verb[:1]) + m.verb[1:]
		run, skip := choice("Run checks, then "+m.verb), choice(title+" without checks")
		chosen, err := choose(context.Background(), s.opts, title, []choice{run, skip}, s.commitInfo(cfg))
		if err != nil || chosen == choiceCancel {
			return err
		}
		if chosen == run {
			failure, err := s.runCommitChecks(cfg.Commit.Checks)
			if errors.Is(err, errStopped) {
				return fmt.Errorf("the checks were stopped, %s", m.outcome)
			}
			if err != nil {
				return err
			}
			if failure != nil {
				// cancelling comes first, Enter must not commit what failed a check
				anyway := choice(title + " anyway")
				chosen, err := choose(context.Background(), s.opts, title, []choice{choiceCancel, anyway}, failureInfo(failure, m.outcome+" yet"))
				if err != nil {
					return err
				}
				if chosen != anyway {
					return fmt.Errorf("%s failed, %s", failure.check.Name, m.outcome)
				}
			}
		}
		return s.bareCommit(m.arg)
	})
}

// runCommitChecks runs the checks on what gets committed: unstaged changes
// are set aside while they run
func (s *status) runCommitChecks(checks []config.Check) (*checkFailure, error) {
	unstaged, err := s.repository.SetAsideUnstaged()
	if err != nil {
		return nil, err
	}
	if unstaged.Any() {
		_, _ = color.New(color.Faint).Fprintln(stdout, "Unstaged changes are set aside until the checks finish.")
	}
	failure, err := runChecks(s.repository.Path(), checks)
	undone, rerr := unstaged.Restore()
	if undone {
		_, _ = color.New(color.FgYellow).Fprintln(stdout, "The checks changed files with unstaged changes, their changes were undone to restore yours.")
	}
	if rerr != nil {
		return nil, rerr
	}
	return failure, err
}

func (s *status) commitInfo(cfg *config.Config) [][]term.Cell {
	grid := [][]term.Cell{checkNames(cfg, cfg.Commit)}
	if s.hasUnstaged() {
		grid = append(grid, term.Cprint("Unstaged changes are set aside while they run, they check what gets committed.", color.Faint))
	}
	return grid
}

func (s *status) hasStaged() bool {
	for _, e := range s.entries {
		if e.Indexed() {
			return true
		}
	}
	return false
}

func (s *status) hasUnstaged() bool {
	for _, e := range s.entries {
		if !e.Indexed() && e.EntryType != git.StatusEntryTypeUntracked && e.EntryType != git.StatusEntryTypeConflicted {
			return true
		}
	}
	return false
}
