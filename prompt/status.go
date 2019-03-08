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
		list: l,
		opts: opts,
	}

	return s.prompt.start(s.innerRun)
}

// this is the main loop for reading input
func (s *Status) innerRun() error {
	var err error

	// start with first render
	s.prompt.render(s.Repo)

	// start waiting for input
	for {
		items, _ := s.prompt.list.Items()
		if len(items) <= 0 && s.Repo.Head != nil {
			defer func() {
				for _, line := range branchClean(s.Repo.Head) {
					s.prompt.writer.Write([]byte(line))
				}
				s.prompt.writer.Flush()
			}()
			err = nil
			break
		}
		r, _, err := s.prompt.reader.ReadRune()
		if err != nil {
			return err
		}
		if r == keys.Interrupt {
			break
		}
		if r == keys.EndTransmission {
			break
		}
		if br := s.assignKey(r); br {
			break
		}
		s.prompt.render(s.Repo)
	}
	// reset cursor position and remove buffer
	s.prompt.writer.Reset()
	s.prompt.writer.ClearScreen()
	return err
}

// assignKey is called on every keypress.
func (s *Status) assignKey(key rune) bool {
	var reqReload bool
	switch key {
	case keys.Enter, '\n':
		s.showDiff()
	case keys.ArrowUp:
		s.prompt.previous()
	case keys.ArrowDown:
		s.prompt.next()
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
	defer s.prompt.render(s.Repo)
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
