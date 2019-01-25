package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/isacikgoz/gitin/git"
	"github.com/isacikgoz/promptui"

	log "github.com/sirupsen/logrus"
)

func Log(r *git.Repository, pos int) error {
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
	prompt := promptui.Select{
		Label:     "Commits",
		Items:     r.Commits,
		HideHelp:  true,
		Searcher:  searcher,
		Templates: templates,
		CanSelect: true,
	}
	// make terminal not line wrap
	fmt.Printf("\x1b[?7l")
	// defer restoring line wrap
	defer fmt.Printf("\x1b[?7h")
	i, _, err := prompt.Run()

	if err != nil {
		return err
	}

	return gitStat(r, r.Commits[i])
}

func gitStat(r *git.Repository, c *git.Commit) error {

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

	ll := func(in interface{}, pos int) error {
		val := diff.Deltas()[pos].PatchString()
		return more(val)
	}
	fp1 := &promptui.FuncPair{
		ll,
		false,
	}

	mm := func(in interface{}, pos int) error {
		return Log(r, pos)
	}
	fp2 := &promptui.FuncPair{
		mm,
		true,
	}

	kset := make(map[rune]*promptui.FuncPair)
	kset[promptui.KeySpace] = fp1
	kset[promptui.KeyBackspace] = fp2

	prompt := promptui.Select{
		Label:       c,
		Items:       diff.Deltas(),
		HideHelp:    true,
		Templates:   templates,
		CustomFuncs: kset,
	}
	_, _, err = prompt.Run()

	return nil
}

func more(in string) error {
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
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
