package cli

import (
	"errors"
	"fmt"

	"github.com/isacikgoz/gitin/git"
	"github.com/isacikgoz/promptui"
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
	if err := r.LoadAll(loadOpts); err != nil {
		return err
	}
	switch opts.Mode {
	case LogNormal:
		commits = r.Commits
	case LogAhead:
		commits = r.Branch.Ahead
	case LogBehind:
		commits = r.Branch.Behind
	case LogMixed:
		commits = r.Branch.Ahead
		commits = append(commits, r.Commits...)
	}
	return logPrompt(r, opts.PromptOps, commits)
}

func logPrompt(r *git.Repository, opts *PromptOptions, commits []*git.Commit) error {
	if len(commits) <= 0 {
		return errors.New("there are no commits to log")
	}
	// make terminal not line wrap
	fmt.Printf("\x1b[?7l")
	// defer restoring line wrap
	defer fmt.Printf("\x1b[?7h")
	var recurse bool
	var prompt promptui.Select
	kset := make(map[promptui.CustomKey]promptui.CustomFunc)
	kset[promptui.CustomKey{Key: 'q', Always: false}] =
		func(in interface{}, chb chan bool, index int) error {
			quitPrompt(r, chb)
			return nil
		}
	kset[promptui.CustomKey{Key: 's', Always: false}] =
		func(in interface{}, chb chan bool, index int) error {
			if err := emuEnterKey(); err != nil {
				chb <- true
			} else {
				chb <- false
			}
			recurse = true
			if err := popGitCmd(r, []string{"show", "--stat", commits[index].Hash}); err != nil {
				return err
			}
			o := currentOptions(&prompt, opts)
			return logPrompt(r, o, commits)
		}
	kset[promptui.CustomKey{Key: 'd', Always: false}] =
		func(in interface{}, chb chan bool, index int) error {
			if err := emuEnterKey(); err != nil {
				chb <- true
			} else {
				chb <- false
			}
			recurse = true
			if err := popGitCmd(r, []string{"diff", commits[index].Hash}); err != nil {
				return err
			}
			o := currentOptions(&prompt, opts)
			return logPrompt(r, o, commits)
		}

	prompt = promptui.Select{
		Label:             "Commits",
		Items:             commits,
		HideHelp:          opts.HideHelp,
		StartInSearchMode: opts.StartInSearch,
		PreSearchString:   opts.InitSearchString,
		Size:              opts.Size,
		SearchLabel:       opts.SearchLabel,
		Searcher:          finderFunc(opts.Finder),
		Templates:         logTemplate(opts.ShowDetail),
		CustomFuncs:       kset,
	}
	i, _, err := prompt.RunCursorAt(opts.Cursor, opts.Scroll)
	if recurse {
		o := currentOptions(&prompt, opts)
		return logPrompt(r, o, commits)
	}

	if err == nil {
		o := &PromptOptions{
			Cursor:   0,
			Scroll:   0,
			Size:     opts.Size,
			HideHelp: opts.HideHelp,
		}
		if err := statPrompt(r, commits[i], o); err != nil && err == NoErrRecurse {
			o := &PromptOptions{
				Cursor:           prompt.CursorPosition(),
				Scroll:           prompt.ScrollPosition(),
				Size:             opts.Size,
				StartInSearch:    prompt.FinishInSearchMode,
				InitSearchString: prompt.PreSearchString,
				HideHelp:         opts.HideHelp,
				Finder:           opts.Finder,
			}
			return logPrompt(r, o, commits)
		}
	}
	return nil
}

func logTemplate(detail bool) *promptui.SelectTemplates {
	templates := &promptui.SelectTemplates{
		Label:    "{{ . |yellow}}:",
		Active:   "* {{ printf \"%.7s\" .Hash | cyan}} {{ .Summary | green}}",
		Inactive: "  {{ printf \"%.7s\" .Hash | cyan}} {{ .Summary}}",
		Selected: "{{ .Summary }}",
		Extra:    "select: enter",
	}
	if detail {
		templates.Details = `
---------------- Commit Detail -----------------
{{ "Hash:"  | faint }}   {{ .Hash | yellow }}
{{ "Author:"| faint }} {{ .Author }}
{{ "Date:"  | faint }}   {{ .Date }} ({{ .Since | blue }})
{{ .CommitRefs }}`
	}
	return templates
}
