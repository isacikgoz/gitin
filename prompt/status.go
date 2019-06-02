package prompt

import (
	"github.com/isacikgoz/fig/git"

	"github.com/isacikgoz/gia/editor"

	"github.com/isacikgoz/sig/keys"
	"github.com/isacikgoz/sig/writer"
)

// Status holds a list of items used to fill the terminal screen.
type Status struct {
	Repo  *git.Repository
	Items []git.FuzzItem

	prompt *prompt
}

// Start draws the screen with its list, initializing the cursor to the given position.
func (s *Status) Start(opts *Options) error {
	l, err := NewList(s.Items, opts.Size)
	if err != nil {
		return err
	}
	s.prompt = &prompt{
		repo:      s.Repo,
		list:      l,
		opts:      opts,
		layout:    status,
		keys:      s.onKey,
		selection: s.onSelect,
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
	case keys.Space:
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
		s.Repo.AddAll()
	case 'r':
		reqReload = true
		s.Repo.ResetAll()
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
	items, err := s.Repo.ReloadStatusEntries()
	if err != nil {
		return err
	}
	s.prompt.list, err = NewList(items, s.prompt.list.Size())
	s.prompt.list.SetCursor(idx)
	return err
}

// add or reset selected entry
func (s *Status) addReset() error {
	defer s.prompt.render()
	items, idx := s.prompt.list.Items()
	item := items[idx].(*git.StatusEntry)
	if item.Indexed() {
		return s.Repo.ResetEntry(item)
	}
	return s.Repo.AddEntry(item)
}

// open hunk stagin ui
func (s *Status) hunkStage() error {
	defer s.prompt.reader.Terminal.Out.Write([]byte(writer.HideCursor))
	items, idx := s.prompt.list.Items()
	entry := items[idx].(*git.StatusEntry)
	file, err := git.GenerateDiffFile(s.Repo, entry)
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
			if err := git.ApplyPatchCmd(s.Repo, entry, patch); err != nil {
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
	return git.PopGenericCmd(s.Repo, entry.FileStatArgs())
}

func (s *Status) doCommit() error {
	defer s.prompt.reader.Terminal.Out.Write([]byte(writer.HideCursor))
	defer s.Repo.Reload()
	args := []string{"commit", "--edit", "--quiet"}
	err := git.PopGenericCmd(s.Repo, args)
	if err != nil {
		return err
	}
	if err := git.PopGenericCmd(s.Repo, s.Repo.LastCommitArgs()); err != nil {
		return err
	}
	return nil
}

func (s *Status) doCommitAmend() error {
	defer s.prompt.reader.Terminal.Out.Write([]byte(writer.HideCursor))
	defer s.Repo.Reload()
	args := []string{"commit", "--amend", "--quiet"}
	err := git.PopGenericCmd(s.Repo, args)
	if err != nil {
		return err
	}
	if err := git.PopGenericCmd(s.Repo, s.Repo.LastCommitArgs()); err != nil {
		return err
	}
	return nil
}
