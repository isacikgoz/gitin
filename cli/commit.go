package cli

import (
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/fatih/color"
	"github.com/isacikgoz/gitin/git"
)

type CommitOptions struct {
	MinCommitLength int
	MaxCommitLength int
	Message         string
	PromptOps       *PromptOptions
}

func CommitBuilder(r *git.Repository, opts *CommitOptions) error {
	if len(opts.Message) <= 0 {
		opts.Message = "message"
	}
	return commitPrompt(r, opts)
}
func commitPrompt(r *git.Repository, opts *CommitOptions) error {
	cmd := exec.Command("git", "commit", "--edit", "--quiet")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	ok := false
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := cmd.Wait(); err == nil {
		ok = true
	}
	if err := r.InitializeStatus(); err != nil {
		return err
	}
	if ok {
		if err := popCommitStat(r.LastCommitHash()); err != nil {
			return err
		}
	}
	return NoErrRecurse
}

func commitAmend(r *git.Repository) error {
	cmd := exec.Command("git", "commit", "--amend", "--quiet")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	ok := false
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := cmd.Wait(); err == nil {
		ok = true
	}
	if err := r.InitializeStatus(); err != nil {
		return err
	}
	if ok {
		if err := popCommitStat(r.LastCommitHash()); err != nil {
			return err
		}
	}
	return NoErrRecurse
}

func popCommitStat(hash string) error {
	os.Setenv("LESS", "-RC")

	cmd := exec.Command("git", "show", "--stat", hash)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Start(); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

func popCommitDiff(hash string) error {
	os.Setenv("LESS", "-RC")

	cmd := exec.Command("git", "diff", hash)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Start(); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

func colorizeStat(input string) string {
	var output string
	yellow := color.New(color.FgYellow)
	re := regexp.MustCompile(`\s\|\s+[\d|Bin|bin]+\s+`)
	rc := regexp.MustCompile(`commit\s+\w{40}`)
	rl := regexp.MustCompile(`\r?\n`)
	rs := regexp.MustCompile(`\d+\s+.+\s+\w+$`)
	lines := rl.Split(input, -1)
	if len(lines) > 0 && rc.MatchString(lines[0]) {
		labelandhash := rc.FindString(lines[0])
		output = output + yellow.Sprint(labelandhash) + "\n"
	}
	for _, line := range lines[1:] {
		if re.MatchString(line) {
			a := re.Split(line, -1)
			file := a[0]
			stat := a[1]
			separator := " | "
			concatStat := strings.Replace(line, a[1], "", 1)
			concatSeparator := strings.Replace(concatStat, separator, "", 1)
			change := strings.Replace(concatSeparator, a[0], "", 1)

			line = file + yellow.Sprint(separator) + change + paintStats(stat, rs)

		}
		output = output + "\n" + line
	}
	return output
}

func paintStats(input string, re *regexp.Regexp) string {
	var output string
	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)
	if re.MatchString(input) {
		output = input
	} else {
		pluses := strings.Count(input, "+")
		minuses := strings.Count(input, "-")
		output = strings.Repeat(green.Sprint("+"), pluses) +
			strings.Repeat(red.Sprint("-"), minuses)
	}

	return output
}
