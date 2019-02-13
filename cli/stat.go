package cli

import (
	"github.com/isacikgoz/gitin/git"
	"github.com/isacikgoz/promptui"
)

func statPrompt(r *git.Repository, c *git.Commit, opts *PromptOptions) error {
	var back bool
	diff, err := r.DiffFromHash(c.Hash)
	if err != nil {
		return err
	}
	var recurse bool
	var prompt promptui.Select
	deltas := diff.Deltas()
	kset := make(map[rune]promptui.CustomFunc)
	kset['q'] = func(in interface{}, chb chan bool, index int) error {
		if err := emuEnterKey(); err != nil {
			chb <- true
		} else {
			chb <- false
		}
		back = true
		return nil
	}
	kset['s'] = func(in interface{}, chb chan bool, index int) error {
		if err := emuEnterKey(); err != nil {
			chb <- true
		} else {
			chb <- false
		}
		recurse = true
		if err = popGitCmd(r, []string{"show", "--stat", c.Hash}); err == NoErrRecurse {
			return nil
		}
		return nil
	}

	prompt = promptui.Select{
		Label:       c,
		Items:       deltas,
		HideHelp:    opts.HideHelp,
		Size:        opts.Size,
		Templates:   statTemplate(c),
		CustomFuncs: kset,
	}
	i, _, err := prompt.RunCursorAt(opts.Cursor, opts.Scroll)
	if recurse {
		o := currentOptions(&prompt, opts)
		return statPrompt(r, c, o)
	}
	if back {
		return NoErrRecurse
	}
	if err == nil {
		o := currentOptions(&prompt, opts)
		if err = popGitCmd(r, deltas[i].FileStatArgs(c)); err == NoErrRecurse {
			return statPrompt(r, c, o)
		}
	}
	return nil
}

func statTemplate(c *git.Commit) *promptui.SelectTemplates {
	templates := &promptui.SelectTemplates{
		Label:    "{{ .Summary | yellow }}",
		Active:   "* {{ .String | green}} ",
		Inactive: "  {{ .String }}",
		Extra:    "stat: s select: enter",
		Details: "\n" +
			"---------------- Commit Detail -----------------" + "\n" +
			"{{ \"Hash:\"   | faint }}   " + "{{ \"" + c.Hash + "\" | yellow }}" + "\n" +
			"{{ \"Author:\" | faint }} " + c.Author.String() + "\n" +
			"{{ \"Date:\"   | faint }}   " + c.Date() + " (" + "{{ \"" + c.Since() + "\" | blue }}" + ")",
	}
	return templates
}
