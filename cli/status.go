package cli

import (
	"fmt"
	"os"
	"strconv"

	"github.com/fatih/color"
	"github.com/isacikgoz/gitin/git"
	"github.com/isacikgoz/promptui"
)

type StatusOptions struct {
	PromptOps *PromptOptions
}

func StatusBuilder(r *git.Repository, opts *StatusOptions) error {
	if err := r.InitializeBranches(); err != nil {
		return err
	}
	return statusPrompt(r, opts.PromptOps)
}
func statusPrompt(r *git.Repository, opts *PromptOptions) error {
	stop := false
	if len(r.Status.Entries) <= 0 {

		yellow := color.New(color.FgYellow)
		fmt.Println("On branch " + yellow.Sprint(r.Branch.Name))
		fmt.Println(getAheadBehind(r.Branch) + "\n")
		fmt.Println("Nothing to commit, working tree clean")
		return nil
	}
	// make terminal not line wrap
	fmt.Printf("\x1b[?7l")
	// defer restoring line wrap
	defer fmt.Printf("\x1b[?7h")

	var prompt promptui.Select
	kset := make(map[rune]promptui.CustomFunc)
	kset['q'] = func(in interface{}, chb chan bool, index int) error {
		chb <- true
		defer os.Exit(0)
		return nil
	}
	kset[' '] = func(in interface{}, chb chan bool, index int) error {
		e := r.Status.Entries[index]
		if e.Indexed() {
			r.ResetEntry(e)
		} else {
			r.AddEntry(e)
		}
		chb <- false
		prompt.RefreshList(r.Status.Entries, index)
		return nil
	}
	kset['a'] = func(in interface{}, chb chan bool, index int) error {
		r.AddAll()
		chb <- false
		prompt.RefreshList(r.Status.Entries, index)
		return nil
	}
	kset['r'] = func(in interface{}, chb chan bool, index int) error {
		r.ResetAll()
		chb <- false
		prompt.RefreshList(r.Status.Entries, index)
		return nil
	}
	kset['m'] = func(in interface{}, chb chan bool, index int) error {
		chb <- true
		opt := &CommitOptions{
			PromptOps: opts,
		}
		err := commitPrompt(r, opt)
		if err != nil && err == NoErrRecurse {
			stop = true
		} else if err != NoErrRecurse {
			os.Exit(0)
		}
		// prompt.RefreshList(r.Status.Entries, index)
		return statusPrompt(r, opts)
	}

	prompt = promptui.Select{
		Label:       "Files",
		Items:       r.Status.Entries,
		HideHelp:    opts.HideHelp,
		Size:        opts.Size,
		Templates:   statusTemplate(r),
		CustomFuncs: kset,
	}
	i, _, err := prompt.RunCursorAt(opts.Cursor, opts.Scroll)

	if stop {
		return nil
	}

	if err == nil {
		o := &PromptOptions{
			Cursor:   prompt.CursorPosition(),
			Scroll:   prompt.ScrollPosition(),
			Size:     opts.Size,
			HideHelp: opts.HideHelp,
		}
		if err := popMore(r.Status.Entries[i].Patch()); err == nil {
			return statusPrompt(r, o)
		}
	}
	return err
}

func getAheadBehind(b *git.Branch) string {
	if b.Upstream == nil || b.Ahead == nil || b.Behind == nil {
		return "Your branch is not tracking a remote branch."
	}
	cyan := color.New(color.FgCyan)
	yellow := color.New(color.FgYellow)
	var str string
	pl := len(b.Behind)
	ps := len(b.Ahead)
	if ps == 0 && pl == 0 {
		str = "Your branch is up to date with " + cyan.Sprint(b.Upstream.Name) + "."
	} else {
		if ps > 0 && pl > 0 {
			str = "Your branch and " + cyan.Sprint(b.Upstream.Name) + " have diverged,"
			str = str + "\n" + "and have " + yellow.Sprint(strconv.Itoa(ps)) + " and " + yellow.Sprint(strconv.Itoa(pl)) + " different commits each, respectively."
			str = str + "\n" + "(\"pull\" to merge the remote branch into yours)"
		} else if pl > 0 && ps == 0 {
			str = "Your branch is behind " + cyan.Sprint(b.Upstream.Name) + " by " + yellow.Sprint(strconv.Itoa(pl)) + " commit(s)."
			str = str + "\n" + "(\"pull\" to update your local branch)"
		} else if ps > 0 && pl == 0 {
			str = "Your branch is ahead of " + cyan.Sprint(b.Upstream.Name) + " by " + yellow.Sprint(strconv.Itoa(ps)) + " commit(s)."
			str = str + "\n" + "(\"push\" to publish your local commits)"
		}
	}
	return str
}

func statusTemplate(r *git.Repository) *promptui.SelectTemplates {
	templates := &promptui.SelectTemplates{
		Label:    "{{ . |yellow}}:",
		Active:   "* {{- if .Indexed }} {{ printf \"%.1s\" .StatusEntryString | green}}{{- else}} {{ printf \"%.1s\" .StatusEntryString | red}}{{- end}} {{ .String }}",
		Inactive: "  {{- if .Indexed }}  {{ printf \"%.1s\" .StatusEntryString | green}}{{- else}}  {{ printf \"%.1s\" .StatusEntryString | red}}{{- end}} {{ .String }}",
		Selected: "{{ .String }}",
		Extra:    "add/reset: space commit: m",
		Details: "\n" +
			"---------------- Status -----------------" + "\n" +
			"{{ \"On branch\" }} " + "{{ \"" + r.Branch.Name + "\" | yellow }}" + "\n" +
			getAheadBehind(r.Branch),
	}
	return templates
}
