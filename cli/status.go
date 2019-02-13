package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"unicode"

	"github.com/fatih/color"
	"github.com/isacikgoz/gitin/editor"
	"github.com/isacikgoz/gitin/git"
	"github.com/isacikgoz/promptui"
	"github.com/waigani/diffparser"

	log "github.com/sirupsen/logrus"
)

// StatusOptions
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
	files, err := generateFileList(r)
	if err != nil {
		return err
	}
	stop := false
	if files == nil || len(files) <= 0 {

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
		quitPrompt(r, chb)
		return nil
	}
	kset[' '] = func(in interface{}, chb chan bool, index int) error {
		e := files[index].Entry()
		if e.Indexed() {
			r.ResetEntry(e)
		} else {
			r.AddEntry(e)
		}
		chb <- false
		var err error
		files, err = generateFileList(r)
		if err != nil {
			return err
		}
		prompt.RefreshList(files, index)
		return nil
	}
	kset['!'] = func(in interface{}, chb chan bool, index int) error {
		e := files[index].Entry()
		args := []string{"checkout", "--", e.String()}
		if err := popGitCmd(r, args); err != nil {
			log.Warn(err)
		}
		files, err = generateFileList(r)
		if files == nil || len(files) <= 0 {
			chb <- true
			stop = true
			return statusPrompt(r, opts)
		}
		chb <- false
		var err error
		if err != nil {
			return err
		}
		prompt.RefreshList(files, index)
		return nil
	}
	kset['a'] = func(in interface{}, chb chan bool, index int) error {
		r.AddAll()
		var err error
		files, err = generateFileList(r)
		if err != nil {
			return err
		}
		chb <- false
		prompt.RefreshList(files, index)
		return nil
	}
	kset['p'] = func(in interface{}, chb chan bool, index int) error {
		stop = true
		if err := emuEnterKey(); err != nil {
			log.Warn(err.Error())
		}
		chb <- true
		entry := files[index].entry
		file, err := generateDiffFile(r, entry)
		if err == nil {
			editor, err := editor.NewEditor(file)
			if err != nil {
				log.Error(err)
				return err
			}
			patches, err := editor.Run()
			if err != nil {
				log.Error(err)
			}
			for _, patch := range patches {
				if err := applyPatch(r, entry, patch); err != nil {
					return err
				}
			}
		} else {
			log.Warn(err.Error())
		}
		o := &PromptOptions{
			Cursor:   prompt.CursorPosition(),
			Scroll:   prompt.ScrollPosition(),
			Size:     opts.Size,
			HideHelp: opts.HideHelp,
		}
		return statusPrompt(r, o)
	}
	kset['r'] = func(in interface{}, chb chan bool, index int) error {
		r.ResetAll()
		var err error
		files, err = generateFileList(r)
		if err != nil {
			return err
		}
		chb <- false
		prompt.RefreshList(files, index)
		return nil
	}
	kset['c'] = func(in interface{}, chb chan bool, index int) error {
		if len(getIndexedEntries(r)) <= 0 {
			return nil
		}
		chb <- true
		opt := &CommitOptions{
			PromptOps: opts,
			Message:   "commit message",
		}
		err := commitPrompt(r, opt)
		if err != nil && err == NoErrRecurse {
			stop = true
		} else if err != NoErrRecurse {
			os.Exit(0)
		}
		return statusPrompt(r, opts)
	}
	kset['m'] = func(in interface{}, chb chan bool, index int) error {
		if len(getIndexedEntries(r)) <= 0 {
			return nil
		}
		chb <- true
		err := commitAmend(r)
		if err != nil && err == NoErrRecurse {
			stop = true
		} else if err != NoErrRecurse {
			os.Exit(0)
		}
		return statusPrompt(r, opts)
	}

	prompt = promptui.Select{
		Label:       "Files",
		Items:       files,
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
		if err := popGitCmd(r, files[i].entry.FileStatArgs()); err == NoErrRecurse {
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
		Active:   "* {{- if .Indexed }} {{ printf \"%.1s\" .Entry.StatusEntryString | green}}{{- else}} {{ printf \"%.1s\" .Entry.StatusEntryString | red}}{{- end}} {{ .Entry.String }}",
		Inactive: "  {{- if .Indexed }}  {{ printf \"%.1s\" .Entry.StatusEntryString | green}}{{- else}}  {{ printf \"%.1s\" .Entry.StatusEntryString | red}}{{- end}} {{ .Entry.String }}",
		Selected: "{{ .Entry.String }}",
		Extra:    "add/reset: space commit: c amend: m patch: p",
		Details: "\n" +
			"---------------- Status -----------------" + "\n" +
			"{{ \"On branch\" }} " + "{{ \"" + r.Branch.Name + "\" | yellow }}" + "\n" +
			getAheadBehind(r.Branch),
	}
	return templates
}

func generateDiffFile(r *git.Repository, entry *git.StatusEntry) (*diffparser.DiffFile, error) {
	args := entry.FileStatArgs()
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	diff, err := diffparser.Parse(string(out))
	if err != nil {
		return nil, err
	}
	return diff.Files[0], nil
}

func applyPatch(r *git.Repository, entry *git.StatusEntry, patch string) error {
	mode := []string{"apply", "--cached"}
	if entry.Indexed() {
		mode = []string{"apply", "--cached", "--reverse"}
	}
	cmd := exec.Command("git", mode...)
	cmd.Dir = r.AbsPath
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	go func() {
		defer stdin.Close()
		io.WriteString(stdin, patch+"\n")
	}()
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Println(string(out))
		log.Error(err.Error())
	}
	return nil
}

// File is wrapper of the git.StatusEntry
type File struct {
	index bool
	entry *git.StatusEntry
}

func generateFileList(r *git.Repository) ([]*File, error) {
	if err := r.InitializeBranches(); err != nil {
		return nil, err
	}
	if err := r.InitializeStatus(); err != nil {
		return nil, err
	}
	files := make([]*File, 0)
	for _, e := range r.Status.Entities {
		if e.Indexed() {

			files = append(files, &File{
				index: true,
				entry: e,
			})

		} else {

			files = append(files, &File{
				entry: e,
			})

		}
	}
	sort.Sort(FilesAlphabetical(files))
	return files, nil
}

func (f *File) Indexed() bool {
	return f.index
}

func (f *File) Entry() *git.StatusEntry {
	return f.entry
}

// FilesAlphabetical slice is the re-ordered *File slice that sorted according
// to alphabetical order (A-Z)
type FilesAlphabetical []*File

// Len is the interface implementation for Alphabetical sorting function
func (s FilesAlphabetical) Len() int { return len(s) }

// Swap is the interface implementation for Alphabetical sorting function
func (s FilesAlphabetical) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// Less is the interface implementation for Alphabetical sorting function
func (s FilesAlphabetical) Less(i, j int) bool {
	iRunes := []rune(s[i].entry.String())
	jRunes := []rune(s[j].entry.String())

	max := len(iRunes)
	if max > len(jRunes) {
		max = len(jRunes)
	}

	for idx := 0; idx < max; idx++ {
		ir := iRunes[idx]
		jr := jRunes[idx]

		lir := unicode.ToLower(ir)
		ljr := unicode.ToLower(jr)

		if lir != ljr {
			return lir < ljr
		}

		// the lowercase runes are the same, so compare the original
		if ir != jr {
			return ir < jr
		}
	}
	return false
}

func getIndexedEntries(r *git.Repository) []*git.StatusEntry {
	files := make([]*git.StatusEntry, 0)
	for _, e := range r.Status.Entities {
		if e.Indexed() {
			files = append(files, e)
		}
	}
	return files
}
