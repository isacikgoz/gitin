package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/isacikgoz/gitin/git"
	"github.com/manifoldco/promptui"

	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetLevel(log.ErrorLevel)
	if err := run("/Users/ibrahim/Development/repositories/gitbatch"); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

}

func run(path string) error {
	r, err := git.Open(path)
	if err != nil {
		return err
	}
	return gitlog(r)
}

func gitlog(r *git.Repository) error {
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
	}
	i, _, err := prompt.Run()

	if err != nil {
		return err
	}

	return gitStat(r, r.Commits[i])
}

func gitStat(r *git.Repository, c *git.Commit) error {

	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "* {{ .String | green}} ",
		Inactive: "  {{ .String }}",
		Selected: "{{ .PatchString }}",
		Details: "\n" +
			"---------------- Commit Detail -----------------" + "\n" +
			"{{ \"Hash:\"   }}   " + "{{ \"" + c.Hash + "\" | yellow }}" + "\n" +
			"{{ \"Author:\" }} " + c.Author.String() + "\n" +
			"{{ \"Date:\"   }}   " + c.Date() + " (" + "{{ \"" + c.Since() + "\" | blue }}" + ")",
	}
	diff, err := r.DiffFromHash(c.Hash)
	if err != nil {
		return err
	}
	prompt := promptui.Select{
		Label:     "",
		Items:     diff.Deltas(),
		HideHelp:  true,
		Templates: templates,
	}
	i, _, err := prompt.Run()

	cmd := exec.Command("less", "-X", "-R", "-F")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	go func() {
		defer stdin.Close()
		io.WriteString(stdin, diff.Deltas()[i].PatchString())
	}()
	return cmd.Run()
}
