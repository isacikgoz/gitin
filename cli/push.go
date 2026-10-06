package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/isacikgoz/gitin/config"
	"github.com/isacikgoz/gitin/git"
	"github.com/isacikgoz/gitin/prompt"
	"github.com/isacikgoz/gitin/term"
)

// choice is an item of the prompts asking what to do
type choice string

const (
	choiceCheckAndPush  choice = "Run checks, then push"
	choiceSkipAndPush   choice = "Push without checks"
	choicePush          choice = "Push"
	choicePushAnyway    choice = "Push anyway"
	choiceCancel        choice = "Cancel"
	maxPushCommitsShown        = 5
)

func (c choice) String() string {
	return string(c)
}

// checkFailure is a check that didn't succeed
type checkFailure struct {
	check config.Check
	err   error
}

// errStopped is returned when the user stops a check with Ctrl-C
var errStopped = errors.New("stopped")

// beforeCheckStart runs right before a check starts, tests use it
var beforeCheckStart = func() {}

// Push pushes the checked out branch. If the configuration file has checks,
// it asks whether to run them first.
func Push(ctx context.Context, r *git.Repository, opts *prompt.Options) error {
	cfg, err := loadConfig(r)
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
	choices := []choice{choicePush}
	if len(cfg.Push.Checks) > 0 {
		choices = []choice{choiceCheckAndPush, choiceSkipAndPush}
	}
	chosen, err := choose(ctx, opts, "Push", choices, pushInfo(target, cfg))
	if err != nil {
		return err
	}
	switch chosen {
	case choiceCancel:
		_, _ = fmt.Fprintln(stdout, "Push cancelled.")
		return nil
	case choiceCheckAndPush:
		failure, err := runChecks(r.Path(), cfg.Push.Checks)
		if errors.Is(err, errStopped) {
			return errors.New("push cancelled")
		}
		if err != nil {
			return err
		}
		if failure != nil {
			// cancelling comes first, Enter must not push what failed a check
			chosen, err := choose(ctx, opts, "Push", []choice{choiceCancel, choicePushAnyway}, failureInfo(failure, "nothing was pushed yet"))
			if err != nil {
				return err
			}
			if chosen != choicePushAnyway {
				return fmt.Errorf("%s failed, nothing was pushed", failure.check.Name)
			}
		}
	}
	return push(r, target)
}

// loadConfig reads the configuration of the repository
func loadConfig(r *git.Repository) (*config.Config, error) {
	commonDir, err := r.CommonDir()
	if err != nil {
		return nil, err
	}
	return config.Load(r.Path(), commonDir)
}

// choose asks the user to pick one of the choices. Quitting the prompt
// returns choiceCancel.
func choose(ctx context.Context, opts *prompt.Options, label string, choices []choice, info [][]term.Cell) (choice, error) {
	list, err := prompt.NewList(choices, len(choices))
	if err != nil {
		return choiceCancel, err
	}
	chosen := choiceCancel
	var p *prompt.Prompt
	p = prompt.Create(label, opts, list,
		prompt.WithSelectionHandler(func(item interface{}) error {
			chosen = item.(choice)
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
		return choiceCancel, err
	}
	if err := p.Run(ctx); err != nil {
		return choiceCancel, err
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
		return append(grid, term.Cprint(fmt.Sprintf("No checks, add them to %s to run them before pushing.", cfg.Personal), color.Faint))
	}
	return append(grid, checkNames(cfg, cfg.Push))
}

// failureInfo describes a failed check, outcome says what didn't happen
func failureInfo(f *checkFailure, outcome string) [][]term.Cell {
	line := term.Cprint(f.check.Name, color.FgYellow)
	line = append(line, term.Cprint(fmt.Sprintf(" failed (%v), %s.", f.err, outcome), color.FgRed)...)
	return [][]term.Cell{line}
}

// checkNames lists the checks of a hook of the configuration
func checkNames(cfg *config.Config, hook config.Hook) []term.Cell {
	var names []string
	for _, check := range hook.Checks {
		names = append(names, check.Name)
	}
	line := term.Cprint(fmt.Sprintf("Checks of %s: ", strings.Join(cfg.Files, " and ")), color.Faint)
	return append(line, term.Cprint(strings.Join(names, ", "))...)
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
		if stopped() {
			return nil, errStopped
		}
		cmd := exec.Command("sh", "-c", check.Run)
		cmd.Dir = dir
		cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr
		beforeCheckStart()
		err := cmd.Start()
		forwarded := false
		if err == nil {
			// Ctrl-C right before the check started didn't reach it
			select {
			case <-interrupts:
				_ = cmd.Process.Signal(os.Interrupt)
				forwarded = true
			default:
			}
			err = cmd.Wait()
		}
		took := duration(time.Since(start))
		// the signal can reach the check before gitin's channel
		if forwarded || interrupted(err) {
			_, _ = color.New(color.FgRed).Fprintf(stdout, "✘ %s stopped after %s\n", check.Name, took)
			return nil, errStopped
		}
		if stopped() {
			return nil, errStopped
		}
		if err != nil {
			_, _ = color.New(color.FgRed).Fprintf(stdout, "✘ %s failed after %s: %v\n", check.Name, took, err)
			return &checkFailure{check: check, err: err}, nil
		}
		_, _ = color.New(color.FgGreen).Fprintf(stdout, "✔ %s passed in %s\n", check.Name, took)
	}
	return nil, nil
}

// interrupted reports whether a command ended because of Ctrl-C
func interrupted(err error) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	return ok && status.Signaled() && status.Signal() == syscall.SIGINT
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
