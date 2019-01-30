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

	Cursor int
	Scroll int
}
type LogMode string

const (
	LogNormal LogMode = "normal"
	LogAhead  LogMode = "ahead"
	LogBehind LogMode = "behind"
	LogMixed  LogMode = "mixed"
)

var (
	NoErrRecurse error = errors.New("catch")
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
	return glog(r, opts.Cursor, opts.Scroll, commits)
}

func glog(r *git.Repository, pos, scroll int, commits []*git.Commit) error {

	templates := &promptui.SelectTemplates{
		Label:    "{{ . |yellow}}:",
		Active:   "* {{ printf \"%.7s\" .Hash | cyan}} {{ .Summary | green}}",
		Inactive: "  {{ printf \"%.7s\" .Hash | cyan}} {{ .Summary}}",
		Selected: "{{ .Summary }}",
		Details: `
---------------- Commit Detail -----------------
{{ "Hash:"  | faint }}   {{ .Hash | yellow }} {{ .Decoration }}
{{ "Author:"| faint }} {{ .Author }}
{{ "Date:"  | faint }}   {{ .Date }} ({{ .Since | blue }})`,
	}
	if len(commits) <= 0 {
		return errors.New("there are no commits to log")
	}
	searcher := func(input string, index int) bool {
		item := commits[index]
		name := strings.Replace(strings.ToLower(item.Message), " ", "", -1)
		input = strings.Replace(strings.ToLower(input), " ", "", -1)

		return strings.Contains(name, input)
	}

	kset := make(map[rune]promptui.CustomFunc)
	kset['q'] = func(in interface{}, chb chan bool, index int) error {
		chb <- true
		defer os.Exit(0)
		return nil
	}

	prompt := promptui.Select{
		Label:       "Commits",
		Items:       commits,
		HideHelp:    false,
		Searcher:    searcher,
		Templates:   templates,
		CustomFuncs: kset,
	}
	// make terminal not line wrap
	fmt.Printf("\x1b[?7l")
	// defer restoring line wrap
	defer fmt.Printf("\x1b[?7h")
	i, _, err := prompt.RunCursorAt(pos, scroll)

	if err == nil {
		if err := gitStat(r, commits[i], 0, 0); err != nil && err == NoErrRecurse {
			return glog(r, prompt.CursorPosition(), prompt.ScrollPosition(), commits)
		}
		return err
	}
	return screenbuf.Clear(os.Stdin)
}
