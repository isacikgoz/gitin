package cli

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"

	"github.com/isacikgoz/gitin/git"
	"github.com/isacikgoz/promptui"
)

func FuzzBuilder(r *git.Repository, opts *PromptOptions) error {
	loadOpts := &git.CommitLoadOptions{}
	r.LoadAll(loadOpts)
	items := make([]git.FuzzItem, 0)
	opts.Size = int(getTermHeight()) - 2
	var wg sync.WaitGroup
	var mx sync.Mutex

	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, b := range r.Branches {
			mx.Lock()
			items = append(items, b)
			mx.Unlock()
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, t := range r.Tags {
			mx.Lock()
			items = append(items, t)
			mx.Unlock()
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, c := range r.Commits {
			mx.Lock()
			items = append(items, c)
			mx.Unlock()
		}
	}()
	wg.Wait()
	return fuzzPrompt(r, opts, items)
}

func fuzzPrompt(r *git.Repository, opts *PromptOptions, items []git.FuzzItem) error {
	// make terminal not line wrap
	fmt.Printf("\x1b[?7l")
	// defer restoring line wrap
	defer fmt.Printf("\x1b[?7h")
	var prompt promptui.Select
	kset := make(map[promptui.CustomKey]promptui.CustomFunc)
	kset[promptui.CustomKey{Key: rune(0x11), Always: true}] =
		func(in interface{}, chb chan bool, index int) error {
			quitPrompt(r, chb)
			return nil
		}
	kset[promptui.CustomKey{Key: rune(0x10), Always: true}] =
		func(in interface{}, chb chan bool, index int) error {
			if c, ok := items[index].(*git.Commit); ok {
				if err := emuEnterKey(); err != nil {
					chb <- true
				} else {
					chb <- false
				}
				if err := popGitCmd(r, []string{"diff", c.Hash}); err != nil {
					return err
				}
			} else {
				chb <- false
			}
			return nil
		}
	prompt = promptui.Select{
		Items:             items,
		HideHelp:          true,
		StartInSearchMode: true,
		HideLabel:         true,
		HideScroll:        true,
		PreSearchString:   opts.InitSearchString,
		SearchLabel:       "",
		Size:              opts.Size,
		Searcher:          specialSearch,
		Templates:         fuzzTemplate(),
		CustomFuncs:       kset,
	}
	i, _, err := prompt.RunCursorAt(opts.Cursor, opts.Scroll)

	if err == nil {
		switch items[i].ShortType() {
		case 'c':
			commit := items[i].(*git.Commit)
			if err := statPrompt(r, commit, opts); err != nil && err == NoErrRecurse {
				o := &PromptOptions{
					Cursor:           prompt.CursorPosition(),
					Scroll:           prompt.ScrollPosition(),
					Size:             opts.Size,
					StartInSearch:    prompt.FinishInSearchMode,
					InitSearchString: prompt.PreSearchString,
					HideHelp:         opts.HideHelp,
					Finder:           opts.Finder,
				}
				return fuzzPrompt(r, o, items)
			}
		}
	}
	return err
}

func fuzzTemplate() *promptui.SelectTemplates {
	templates := &promptui.SelectTemplates{
		Active:   "[{{ printf \"%c\" .ShortType | cyan }}] {{ .Display | green}}",
		Inactive: "[{{ printf \"%c\" .ShortType | cyan }}] {{ .Display}}",
		Selected: "{{ .Display }}",
	}
	return templates
}

type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

func getTermHeight() uint {
	ws := &winsize{}
	retCode, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(syscall.Stdin),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)))

	if int(retCode) == -1 {
		panic(errno)
	}
	return uint(ws.Row)
}
