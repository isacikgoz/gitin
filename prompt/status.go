package prompt

import (
	"os/exec"
	"strconv"

	"github.com/isacikgoz/gia/editor"
	git "github.com/isacikgoz/libgit2-api"
)

// Status holds a list of items used to fill the terminal screen.
type Status struct {
	Repo *git.Repository

	prompt *prompt
}

// Start draws the screen with its list, initializing the cursor to the given position.
func (s *Status) Start(opts *Options) error {
	st, err := s.Repo.LoadStatus()
	if err != nil {
		return err
	}
	items := make([]Item, 0)
	for _, entry := range st.Entities {
		items = append(items, entry)
	}
	l, err := NewList(items, opts.Size)
	if err != nil {
		return err
	}
	controls := make(map[string]string)
	controls["add/reset entry"] = "space"
	controls["show diff"] = "enter"
	controls["add all"] = "a"
	controls["reset all"] = "r"
	controls["hunk stage"] = "p"
	controls["commit"] = "c"
	controls["amend"] = "m"

	opts.SearchLabel = "Files"

	s.prompt = &prompt{
		repo:      s.Repo,
		list:      l,
		opts:      opts,
		layout:    status,
		keys:      s.onKey,
		selection: s.onSelect,
		info:      s.branchInfo,
		controls:  controls,
	}

	return s.prompt.start()
}

// return true to terminate
func (s *Status) onSelect() bool {
	s.showDiff()
	return false
}

func (s *Status) onKey(key rune) bool {
	var reqReload bool

	switch key {
	case ' ':
		reqReload = true
		s.addReset()
	case 'p':
		reqReload = true
		s.hunkStage()
	case 'c':
		reqReload = true
		s.doCommit()
	case 'm':
		reqReload = true
		s.doCommitAmend()
	case 'a':
		reqReload = true
		// TODO: check for errors
		addAll(s.Repo)
	case 'r':
		reqReload = true
		resetAll(s.Repo)
	case 'q':
		return true
	default:
	}
	if reqReload {
		s.reloadStatus()
	}
	return false
}

// reloads the list
func (s *Status) reloadStatus() error {
	_, idx := s.prompt.list.Items()
	status, err := s.Repo.LoadStatus()
	if err != nil {
		return err
	}
	items := make([]Item, 0)
	for _, entry := range status.Entities {
		items = append(items, entry)
	}
	s.prompt.list, err = NewList(items, s.prompt.list.size)
	if err != nil {
		return err
	}
	s.prompt.list.SetCursor(idx)
	return nil
}

// add or reset selected entry
func (s *Status) addReset() error {
	defer s.prompt.render()
	items, idx := s.prompt.list.Items()
	entry := items[idx].(*git.StatusEntry)
	args := []string{"add", "--", entry.String()}
	if entry.Indexed() {
		args = []string{"reset", "HEAD", "--", entry.String()}
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = s.Repo.Path()
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// open hunk stagin ui
func (s *Status) hunkStage() error {
	defer s.prompt.reader.Terminal.HideCursor()
	items, idx := s.prompt.list.Items()
	entry := items[idx].(*git.StatusEntry)
	file, err := generateDiffFile(s.Repo, entry)
	if err == nil {
		editor, err := editor.NewEditor(file)
		if err != nil {
			return err
		}
		patches, err := editor.Run()
		if err != nil {
			return err
		}
		for _, patch := range patches {
			if err := applyPatchCmd(s.Repo, entry, patch); err != nil {
				return err
			}
		}
	} else {

	}
	return nil
}

// pop git diff
func (s *Status) showDiff() error {
	items, idx := s.prompt.list.Items()
	entry := items[idx].(*git.StatusEntry)
	return popGitCommand(s.Repo, fileStatArgs(entry))
}

func (s *Status) doCommit() error {
	defer s.prompt.reader.Terminal.HideCursor()

	args := []string{"commit", "--edit", "--quiet"}
	err := popGitCommand(s.Repo, args)
	if err != nil {
		return err
	}
	args, err = lastCommitArgs(s.Repo)
	if err != nil {
		return err
	}
	if err := popGitCommand(s.Repo, args); err != nil {
		return err
	}
	return nil
}

func (s *Status) doCommitAmend() error {
	defer s.prompt.reader.Terminal.HideCursor()

	args := []string{"commit", "--amend", "--quiet"}
	err := popGitCommand(s.Repo, args)
	if err != nil {
		return err
	}
	args, err = lastCommitArgs(s.Repo)
	if err != nil {
		return err
	}
	if err := popGitCommand(s.Repo, args); err != nil {
		return err
	}
	return nil
}

func (s *Status) branchInfo(item Item) []string {
	b := s.Repo.Head
	if b == nil {
		return []string{faint.Sprint("Unable to load branch info")}
	}

	var str []string
	if len(b.Name) > 0 {
		str = []string{faint.Sprint("On branch ") + yellow.Sprint(b.Name)}
	}
	if b.Upstream == nil {
		return append(str, faint.Sprint("Your branch is not tracking a remote branch."))
	}
	pl := b.Behind
	ps := b.Ahead

	if ps == 0 && pl == 0 {
		str = append(str, faint.Sprint("Your branch is up to date with ")+cyan.Sprint(b.Upstream.Name)+faint.Sprint("."))
	} else {
		if ps > 0 && pl > 0 {
			str = append(str, faint.Sprint("Your branch and ")+cyan.Sprint(b.Upstream.Name)+faint.Sprint(" have diverged,"))
			str = append(str, faint.Sprint("and have ")+yellow.Sprint(strconv.Itoa(ps))+faint.Sprint(" and ")+yellow.Sprint(strconv.Itoa(pl))+faint.Sprint(" different commits each, respectively."))
			str = append(str, faint.Sprint("(\"pull\" to merge the remote branch into yours)"))
		} else if pl > 0 && ps == 0 {
			str = append(str, faint.Sprint("Your branch is behind ")+cyan.Sprint(b.Upstream.Name)+faint.Sprint(" by ")+yellow.Sprint(strconv.Itoa(pl))+faint.Sprint(" commit(s)."))
			str = append(str, faint.Sprint("(\"pull\" to update your local branch)"))
		} else if ps > 0 && pl == 0 {
			str = append(str, faint.Sprint("Your branch is ahead of ")+cyan.Sprint(b.Upstream.Name)+faint.Sprint(" by ")+yellow.Sprint(strconv.Itoa(ps))+faint.Sprint(" commit(s)."))
			str = append(str, faint.Sprint("(\"push\" to publish your local commits)"))
		}
	}
	return str
}
