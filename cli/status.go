package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"

	"github.com/fatih/color"
	"github.com/isacikgoz/gitin/git"
	"github.com/isacikgoz/promptui"
	"github.com/isacikgoz/promptui/screenbuf"
	log "github.com/sirupsen/logrus"
)

func Status(r *git.Repository, pos int) error {
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
	templates := &promptui.SelectTemplates{
		Label:    "{{ . |yellow}}:",
		Active:   "* {{- if .Indexed }} {{ printf \"%.1s\" .StatusEntryString | green}}{{- else}} {{ printf \"%.1s\" .StatusEntryString | red}}{{- end}} {{ .String }}",
		Inactive: "  {{- if .Indexed }}  {{ printf \"%.1s\" .StatusEntryString | green}}{{- else}}  {{ printf \"%.1s\" .StatusEntryString | red}}{{- end}} {{ .String }}",
		Selected: "{{ .String }}",
		Details: "\n" +
			"---------------- Status -----------------" + "\n" +
			"{{ \"On branch\" }} " + "{{ \"" + r.Branch.Name + "\" | yellow }}" + "\n" +
			getAheadBehind(r.Branch),
	}
	var prompt promptui.Select
	kset := make(map[rune]promptui.CustomFunc)
	kset['q'] = func(in interface{}, chb chan bool, pos int) error {
		chb <- true
		defer os.Exit(0)
		return nil
	}
	kset[' '] = func(in interface{}, chb chan bool, pos int) error {
		e := r.Status.Entries[pos]
		if e.Indexed() {
			r.ResetEntry(e)
		} else {
			r.AddEntry(e)
		}
		chb <- false
		prompt.RefreshList(r.Status.Entries, pos)
		return nil
	}
	kset['a'] = func(in interface{}, chb chan bool, pos int) error {
		r.AddAll()
		chb <- false
		prompt.RefreshList(r.Status.Entries, pos)
		return nil
	}
	kset['r'] = func(in interface{}, chb chan bool, pos int) error {
		r.ResetAll()
		chb <- false
		prompt.RefreshList(r.Status.Entries, pos)
		return nil
	}

	prompt = promptui.Select{
		Label:       "Files",
		Items:       r.Status.Entries,
		HideHelp:    true,
		Templates:   templates,
		CustomFuncs: kset,
	}
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
