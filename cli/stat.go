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

func gitStat(r *git.Repository, c *git.Commit, pos, scroll int) error {
	var back bool
	templates := &promptui.SelectTemplates{
		Label:    "{{ .Summary | yellow }}",
		Active:   "* {{ .String | green}} ",
		Inactive: "  {{ .String }}",
		Details: "\n" +
			"---------------- Commit Detail -----------------" + "\n" +
			"{{ \"Hash:\"   | faint }}   " + "{{ \"" + c.Hash + "\" | yellow }}" + "\n" +
			"{{ \"Author:\" | faint }} " + c.Author.String() + "\n" +
			"{{ \"Date:\"   | faint }}   " + c.Date() + " (" + "{{ \"" + c.Since() + "\" | blue }}" + ")",
	}
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
		HideHelp:    false,
		Templates:   templates,
		CustomFuncs: kset,
	}
	i, _, err := prompt.RunCursorAt(pos, scroll)
	if back {
		return NoErrRecurse
	}
	if err == nil {
		return logmore(r, c, diff.Deltas()[i].PatchString(), prompt.CursorPosition(), prompt.ScrollPosition())
	}
	return screenbuf.Clear(os.Stdin)
}

func logmore(r *git.Repository, c *git.Commit, in string, pos, scroll int) error {
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
	if err := cmd.Wait(); err == nil {
		return gitStat(r, c, pos, scroll)
	}
	return nil
}
