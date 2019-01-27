package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/isacikgoz/gitin/git"
	"github.com/isacikgoz/promptui"
	"github.com/isacikgoz/promptui/screenbuf"
	log "github.com/sirupsen/logrus"
)

func Status(r *git.Repository, pos int) error {
	templates := &promptui.SelectTemplates{
		Label:    "{{ . |yellow}}:",
		Active:   "* {{- if .Indexed }} {{ printf \"%.1s\" .StatusEntryString | green}}{{- else}} {{ printf \"%.1s\" .StatusEntryString | red}}{{- end}} {{ .String }}",
		Inactive: "  {{- if .Indexed }}  {{ printf \"%.1s\" .StatusEntryString | green}}{{- else}}  {{ printf \"%.1s\" .StatusEntryString | red}}{{- end}} {{ .String }}",
		Selected: "{{ .String }}",
		Details: "\n" +
			"---------------- Status -----------------" + "\n" +
			"{{ \"Branch:\"   | faint }}   " + "{{ \"" + r.Branch.Name + "\" | yellow }}" + "\n" +
			"{{ \"Upstream:\"   | faint }} " + "{{ \"" + r.Branch.Upstream.FullName + "\" | yellow }}" + "\n",
	}
	var prompt promptui.Select
	qfunc := func(in interface{}, chb chan bool, pos int) error {
		chb <- true
		defer os.Exit(0)
		return nil
	}
	afunc := func(in interface{}, chb chan bool, pos int) error {
		e := r.Status.Entries[pos]
		if e.Indexed() {
			r.ResetEntry(e)
		} else {
			r.AddEntry(e)
		}
		chb <- false
		prompt.RefreshList(r.Status.Entries)

		return nil
	}
	kset := make(map[rune]promptui.CustomFunc)
	kset['q'] = qfunc
	kset[' '] = afunc

	prompt = promptui.Select{
		Label:       "Status",
		Items:       r.Status.Entries,
		HideHelp:    true,
		Templates:   templates,
		CustomFuncs: kset,
	}
	// make terminal not line wrap
	fmt.Printf("\x1b[?7l")
	// defer restoring line wrap
	defer fmt.Printf("\x1b[?7h")
	i, _, err := prompt.Run()

	if err == nil {
		return statusmore(r, r.Status.Entries[i].Patch())
	}
	return screenbuf.Clear(os.Stdin)
}

func statusmore(r *git.Repository, in string) error {
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
		return Status(r, 0)
	}
	return nil
}
