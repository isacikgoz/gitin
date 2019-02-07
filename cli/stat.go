package cli

import (
	"os"

	"github.com/isacikgoz/gitin/git"
	"github.com/isacikgoz/promptui"
	"github.com/isacikgoz/promptui/screenbuf"
)

func statPrompt(r *git.Repository, c *git.Commit, opts *PromptOptions) error {
	var back bool
	diff, err := r.DiffFromHash(c.Hash)
	if err != nil {
		return err
	}
	deltas := diff.Deltas()
	kset := make(map[rune]promptui.CustomFunc)
	kset['q'] = func(in interface{}, chb chan bool, index int) error {
		screenbuf.Clear(os.Stdin)
		if err := emuEnterKey(); err != nil {
			chb <- true
		} else {
			chb <- false
		}
		back = true
		return nil
	}

	prompt := promptui.Select{
		Label:       c,
		Items:       deltas,
		HideHelp:    opts.HideHelp,
		Size:        opts.Size,
		Templates:   statTemplate(c),
		CustomFuncs: kset,
	}
	i, _, err := prompt.RunCursorAt(opts.Cursor, opts.Scroll)
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
		Extra:    "select: enter",
		Details: "\n" +
			"---------------- Commit Detail -----------------" + "\n" +
			"{{ \"Hash:\"   | faint }}   " + "{{ \"" + c.Hash + "\" | yellow }}" + "\n" +
			"{{ \"Author:\" | faint }} " + c.Author.String() + "\n" +
			"{{ \"Date:\"   | faint }}   " + c.Date() + " (" + "{{ \"" + c.Since() + "\" | blue }}" + ")",
	}
	return templates
}
