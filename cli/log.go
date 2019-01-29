package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/isacikgoz/gitin/git"
	"github.com/isacikgoz/promptui"
	"github.com/isacikgoz/promptui/screenbuf"

	log "github.com/sirupsen/logrus"
)

func Log(r *git.Repository, pos, scroll int) error {
	templates := &promptui.SelectTemplates{
		Label:    "{{ . |yellow}}:",
		Active:   "* {{ printf \"%.7s\" .Hash | cyan}} {{ .Summary | green}}",
		Inactive: "  {{ printf \"%.7s\" .Hash | cyan}} {{ .Summary}}",
		Selected: "{{ .Summary }}",
		Details: `
---------------- Commit Detail -----------------
{{ "Hash:"  | faint }}   {{ .Hash | yellow }}
{{ "Author:"| faint }} {{ .Author }}
{{ "Date:"  | faint }}   {{ .Date }} ({{ .Since | blue }})`,
	}

	searcher := func(input string, index int) bool {
		item := r.Commits[index]
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
		Items:       r.Commits,
		HideHelp:    true,
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
		return gitStat(r, r.Commits[i], 0, 0, prompt.CursorPosition(), prompt.ScrollPosition())
	}
	return screenbuf.Clear(os.Stdin)
}

func gitStat(r *git.Repository, c *git.Commit, pos, scroll int, parentpos, parentscroll int) error {

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
		return Log(r, parentpos, parentscroll)
	}

	prompt := promptui.Select{
		Label:       c,
		Items:       diff.Deltas(),
		HideHelp:    true,
		Templates:   templates,
		CustomFuncs: kset,
	}
	i, _, err := prompt.RunCursorAt(pos, scroll)
	if err == nil {
		return logmore(r, c, diff.Deltas()[i].PatchString(), prompt.CursorPosition(), prompt.ScrollPosition(), parentpos, parentscroll)
	}
	return screenbuf.Clear(os.Stdin)
}

func logmore(r *git.Repository, c *git.Commit, in string, pos, scroll int, parentpos, parentscroll int) error {
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
		return gitStat(r, c, pos, scroll, parentpos, parentscroll)
	}
	return nil
}
