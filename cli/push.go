package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/isacikgoz/gitin/config"
	"github.com/isacikgoz/gitin/git"
	"github.com/isacikgoz/gitin/prompt"
	"github.com/isacikgoz/gitin/term"
)

// pushAction is a choice of the push prompt
type pushAction string

const (
	actionCheckAndPush  pushAction = "Run checks, then push"
	actionSkipAndPush   pushAction = "Push without checks"
	actionPush          pushAction = "Push"
	actionPushAnyway    pushAction = "Push anyway"
	actionCancel        pushAction = "Cancel"
	maxPushCommitsShown            = 5
)

func (a pushAction) String() string {
	return string(a)
}

// checkFailure is a check that didn't succeed
type checkFailure struct {
	check config.Check
	err   error
}

// errInterrupted is returned when the user stops a check with Ctrl-C
var errInterrupted = errors.New("push cancelled")

// Push pushes the checked out branch. If the configuration file has checks,
// it asks whether to run them first.
func Push(ctx context.Context, r *git.Repository, opts *prompt.Options) error {
	cfg, err := config.Load(r.Path())
	if err != nil {
		return err
	}
	target, err := r.PushTarget()
	if err != nil {
		return err
	}
	if target.Ahead == 0 && !target.SetUpstream {
		_, _ = fmt.Fprintln(stdout, nothingToPush(target))
		return nil
	}
	if opts.DisableColor {
		color.NoColor = true
	}

	// q and Ctrl-C cancel, like on every gitin screen
	actions := []pushAction{actionPush}
	if len(cfg.Push.Checks) > 0 {
		actions = []pushAction{actionCheckAndPush, actionSkipAndPush}
	}
	action, err := choose(ctx, opts, actions, pushInfo(target, cfg))
	if err != nil {
		return err
	}
	switch action {
	case actionCancel:
		_, _ = fmt.Fprintln(stdout, "Push cancelled.")
		return nil
	case actionCheckAndPush:
		failure, err := runChecks(r.Path(), cfg.Push.Checks)
		if err != nil {
			return err
		}
		if failure != nil {
			// cancelling comes first, Enter must not push what failed a check
			action, err := choose(ctx, opts, []pushAction{actionCancel, actionPushAnyway}, failureInfo(failure))
			if err != nil {
				return err
			}
			if action != actionPushAnyway {
				return fmt.Errorf("%s failed, nothing was pushed", failure.check.Name)
			}
		}
	}
	return push(r, target)
}

// choose asks the user to pick one of the actions. Quitting the prompt cancels.
func choose(ctx context.Context, opts *prompt.Options, actions []pushAction, info [][]term.Cell) (pushAction, error) {
	list, err := prompt.NewList(actions, len(actions))
	if err != nil {
		return actionCancel, err
	}
	chosen := actionCancel
	var p *prompt.Prompt
	p = prompt.Create("Push", opts, list,
		prompt.WithSelectionHandler(func(item interface{}) error {
			chosen = item.(pushAction)
			p.Stop()
			return nil
		}),
		prompt.WithItemRenderer(renderItem),
		prompt.WithInformation(func(interface{}) [][]term.Cell { return info }),
	)
	quit := &prompt.KeyBinding{Key: 'q', Display: "q", Desc: "cancel", Handler: func(interface{}) error {
		p.Stop()
		return nil
	}}
	if err := p.AddKeyBinding(quit); err != nil {
		return actionCancel, err
	}
	if err := p.Run(ctx); err != nil {
		return actionCancel, err
	}
	return chosen, nil
}

// pushInfo describes what gets pushed and which checks can run
func pushInfo(t *git.PushTarget, cfg *config.Config) [][]term.Cell {
	var grid [][]term.Cell
	commits := term.Cprint(fmt.Sprintf("%d %s", t.Ahead, plural(t.Ahead, "commit")), color.FgYellow)
	var line []term.Cell
	if t.SetUpstream {
		line = append(term.Cprint("New branch ", color.Faint), term.Cprint(t.Branch, color.FgYellow)...)
		line = append(line, term.Cprint(" is pushed to ", color.Faint)...)
		line = append(line, term.Cprint(t.RemoteBranch, color.FgCyan)...)
		line = append(line, term.Cprint(" with ", color.Faint)...)
	} else {
		line = append(term.Cprint(t.Branch, color.FgYellow), term.Cprint(" → ", color.Faint)...)
		line = append(line, term.Cprint(t.RemoteBranch, color.FgCyan)...)
		line = append(line, term.Cprint(", ", color.Faint)...)
	}
	line = append(line, commits...)
	grid = append(grid, append(line, term.Cprint(":", color.Faint)...))

	for i, c := range t.Commits {
		if i == maxPushCommitsShown {
			break
		}
		line := append(term.Cprint("  "), stautsText(c.Hash[:7])...)
		grid = append(grid, append(line, term.Cprint(c.Summary)...))
	}
	if more := t.Ahead - min(len(t.Commits), maxPushCommitsShown); more > 0 {
		grid = append(grid, term.Cprint(fmt.Sprintf("  and %d more", more), color.Faint))
	}
	if t.Behind > 0 {
		grid = append(grid, term.Cprint(fmt.Sprintf("%s has %d %s %s doesn't have, the push is rejected until you pull %s.",
			t.RemoteBranch, t.Behind, plural(t.Behind, "commit"), t.Branch, map[bool]string{true: "it", false: "them"}[t.Behind == 1]), color.FgRed))
	}

	if len(cfg.Push.Checks) == 0 {
		return append(grid, term.Cprint(fmt.Sprintf("No checks, add them to %s to run them before pushing.", config.FileNames[0]), color.Faint))
	}
	var names []string
	for _, check := range cfg.Push.Checks {
		names = append(names, check.Name)
	}
	line = term.Cprint(fmt.Sprintf("Checks of %s: ", cfg.File), color.Faint)
	return append(grid, append(line, term.Cprint(strings.Join(names, ", "))...))
}

func failureInfo(f *checkFailure) [][]term.Cell {
	line := term.Cprint(f.check.Name, color.FgYellow)
	line = append(line, term.Cprint(fmt.Sprintf(" failed (%v), nothing was pushed yet.", f.err), color.FgRed)...)
	return [][]term.Cell{line}
}

func nothingToPush(t *git.PushTarget) string {
	if t.Behind > 0 {
		return fmt.Sprintf("Nothing to push, %s is behind %s by %d %s.", t.Branch, t.RemoteBranch, t.Behind, plural(t.Behind, "commit"))
	}
	return fmt.Sprintf("Nothing to push, %s is up to date with %s.", t.Branch, t.RemoteBranch)
}

// runChecks runs the checks in dir one after the other, their output goes
// to the terminal. It returns the first check that fails.
func runChecks(dir string, checks []config.Check) (*checkFailure, error) {
	// Ctrl-C stops the running check, the terminal sends the signal to it
	// as well, and gitin cancels the push instead of exiting
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	defer signal.Stop(interrupts)

	bold, faint := color.New(color.Bold), color.New(color.Faint)
	for _, check := range checks {
		_, _ = bold.Fprintf(stdout, "▶ %s", check.Name)
		if check.Name != check.Run {
			_, _ = faint.Fprintf(stdout, "  %s", firstLine(check.Run))
		}
		_, _ = fmt.Fprintln(stdout)

		start := time.Now()
		stopped := func() bool {
			select {
			case <-interrupts:
				_, _ = color.New(color.FgRed).Fprintf(stdout, "✘ %s stopped after %s\n", check.Name, duration(time.Since(start)))
				return true
			default:
				return false
			}
		}
		// Ctrl-C before the check started didn't reach it
		if stopped() {
			return nil, errInterrupted
		}
		cmd := exec.Command("sh", "-c", check.Run)
		cmd.Dir = dir
		cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr
		err := cmd.Run()
		took := duration(time.Since(start))
		if stopped() {
			return nil, errInterrupted
		}
		if err != nil {
			_, _ = color.New(color.FgRed).Fprintf(stdout, "✘ %s failed after %s: %v\n", check.Name, took, err)
			return &checkFailure{check: check, err: err}, nil
		}
		_, _ = color.New(color.FgGreen).Fprintf(stdout, "✔ %s passed in %s\n", check.Name, took)
	}
	return nil, nil
}

// firstLine returns the first line of a command, with … if it has more lines
func firstLine(command string) string {
	first, rest, _ := strings.Cut(strings.TrimSpace(command), "\n")
	if rest != "" {
		return first + " …"
	}
	return first
}

// duration formats d like 35ms, 3.2s or 1m2.3s
func duration(d time.Duration) string {
	if d < time.Second {
		return d.Round(time.Millisecond).String()
	}
	return d.Round(100 * time.Millisecond).String()
}

// push runs git push on the terminal, e.g. it can ask for credentials
func push(r *git.Repository, t *git.PushTarget) error {
	_, _ = color.New(color.Bold).Fprintf(stdout, "▶ Pushing %s to %s\n", t.Branch, t.RemoteBranch)
	cmd := r.Command(t.Args()...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr
	if err := cmd.Run(); err != nil {
		return errors.New("git push failed") // git printed why
	}
	return nil
}
