package cli

import (
	"io"
	"os"
	"os/exec"

	"github.com/isacikgoz/gitin/git"
	"github.com/isacikgoz/promptui"
	"github.com/isacikgoz/promptui/screenbuf"

	log "github.com/sirupsen/logrus"
)

func statPrompt(r *git.Repository, c *git.Commit, opts *PromptOptions) error {
	var back bool
	diff, err := r.DiffFromHash(c.Hash)
	if err != nil {
		return err
	}

	kset := make(map[rune]promptui.CustomFunc)
	kset['q'] = func(in interface{}, chb chan bool, index int) error {
		screenbuf.Clear(os.Stdin)
		chb <- true
		back = true
		return nil
	}

	prompt := promptui.Select{
		Label:       c,
		Items:       diff.Deltas(),
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
		o := &PromptOptions{
			Cursor:   prompt.CursorPosition(),
			Scroll:   prompt.ScrollPosition(),
			Size:     opts.Size,
			HideHelp: opts.HideHelp,
		}
		if err = popLess(r, c, diff.Deltas()[i].PatchString()); err == nil {
			return statPrompt(r, c, o)
		}
	}
	return nil
}

func popLess(r *git.Repository, c *git.Commit, in string) error {
	os.Setenv("LESS", "-RC")
	cmd := exec.Command("less")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	defer func() {
		cmd.Stdin = os.Stdin
	}()
	go func() {
		defer stdin.Close()
		io.WriteString(stdin, in)
	}()
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		// return statPrompt(r, c, opts)
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
