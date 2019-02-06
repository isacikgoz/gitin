package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/isacikgoz/gitin/git"
	"github.com/isacikgoz/promptui"
	"github.com/isacikgoz/promptui/screenbuf"
)

type LogOptions struct {
	Mode      LogMode
	Author    string
	Before    string
	Committer string
	MaxCount  int
	Tags      bool
	Since     string

	PromptOps *PromptOptions
}
type LogMode uint8

const (
	LogNormal LogMode = iota
	LogAhead
	LogBehind
	LogMixed
)

func LogBuilder(r *git.Repository, opts *LogOptions) error {
	var commits []*git.Commit
	loadOpts := &git.CommitLoadOptions{
		MaxCount:  opts.MaxCount,
		Author:    opts.Author,
		Committer: opts.Committer,
		Since:     opts.Since,
		Before:    opts.Before,
	}
	if opts.Tags {
		if err := r.InitializeTags(); err != nil {
			return err
		}
	}

	switch opts.Mode {
	case LogNormal:
		if err := r.InitializeCommits(loadOpts); err != nil {
			return err
		}
		commits = r.Commits
	case LogAhead:
		if err := r.InitializeBranches(); err != nil {
			return err
		}
		commits = r.Branch.Ahead
	case LogBehind:
		if err := r.InitializeBranches(); err != nil {
			return err
		}
		commits = r.Branch.Behind
	case LogMixed:
		if err := r.InitializeBranches(); err != nil {
			return err
		}
		if err := r.InitializeCommits(loadOpts); err != nil {
			return err
		}
		commits = r.Branch.Ahead
		commits = append(commits, r.Commits...)
	}
	return logPrompt(r, opts.PromptOps, commits)
}

func logPrompt(r *git.Repository, opts *PromptOptions, commits []*git.Commit) error {
	if len(commits) <= 0 {
		return errors.New("there are no commits to log")
	}
	searcher := func(input string, index int) bool {
		item := commits[index]
		name := strings.Replace(strings.ToLower(item.Message), " ", "", -1)
		input = strings.Replace(strings.ToLower(input), " ", "", -1)

		return strings.Contains(name, input)
	}
	var prompt promptui.Select
	kset := make(map[rune]promptui.CustomFunc)
	kset['q'] = func(in interface{}, chb chan bool, index int) error {
		chb <- true
		defer os.Exit(0)
		return nil
	}
	kset['s'] = func(in interface{}, chb chan bool, index int) error {
		screenbuf.Clear(os.Stdin)
		if err := popGitCmd(r, []string{"show", "--stat", commits[index].Hash}); err != nil {
			return err
		}
		chb <- true
		o := &PromptOptions{
			Cursor:   prompt.CursorPosition(),
			Scroll:   prompt.ScrollPosition(),
			Size:     opts.Size,
			HideHelp: opts.HideHelp,
		}
		return logPrompt(r, o, commits)
	}
	kset['d'] = func(in interface{}, chb chan bool, index int) error {
		screenbuf.Clear(os.Stdin)
		if err := popGitCmd(r, []string{"diff", commits[index].Hash}); err != nil {
			return err
		}
		chb <- true
		o := &PromptOptions{
			Cursor:   prompt.CursorPosition(),
			Scroll:   prompt.ScrollPosition(),
			Size:     opts.Size,
			HideHelp: opts.HideHelp,
		}
		return logPrompt(r, o, commits)
	}

	prompt = promptui.Select{
		Label:       "Commits",
		Items:       commits,
		HideHelp:    opts.HideHelp,
		Size:        opts.Size,
		Searcher:    searcher,
		Templates:   logTemplate(),
		CustomFuncs: kset,
	}
	// make terminal not line wrap
	fmt.Printf("\x1b[?7l")
	// defer restoring line wrap
	defer fmt.Printf("\x1b[?7h")
	i, _, err := prompt.RunCursorAt(opts.Cursor, opts.Scroll)

	if err == nil {
		o := &PromptOptions{
			Cursor:   0,
			Scroll:   0,
			Size:     opts.Size,
			HideHelp: opts.HideHelp,
		}
		if err := statPrompt(r, commits[i], o); err != nil && err == NoErrRecurse {
			o := &PromptOptions{
				Cursor:   prompt.CursorPosition(),
				Scroll:   prompt.ScrollPosition(),
				Size:     opts.Size,
				HideHelp: opts.HideHelp,
			}
			return logPrompt(r, o, commits)
		}
	}
	return nil
}

func logTemplate() *promptui.SelectTemplates {
	templates := &promptui.SelectTemplates{
		Label:    "{{ . |yellow}}:",
		Active:   "* {{ printf \"%.7s\" .Hash | cyan}} {{ .Summary | green}}",
		Inactive: "  {{ printf \"%.7s\" .Hash | cyan}} {{ .Summary}}",
		Selected: "{{ .Summary }}",
		Extra:    "select: enter",
		Details: `
---------------- Commit Detail -----------------
{{ "Hash:"  | faint }}   {{ .Hash | yellow }} {{ .Decoration }}
{{ "Author:"| faint }} {{ .Author }}
{{ "Date:"  | faint }}   {{ .Date }} ({{ .Since | blue }})`,
	}
	return templates
}
