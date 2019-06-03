package prompt

import (
	"github.com/isacikgoz/gia/editor"
	git "github.com/isacikgoz/libgit2-api"

	"github.com/isacikgoz/sig/keys"
	"github.com/isacikgoz/sig/writer"
)

// Status holds a list of items used to fill the terminal screen.
type Status struct {
	Repo  *git.Repository
	Items []Item

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
	case 'k':
		s.prompt.list.Prev()
	case 'j':
		s.prompt.list.Next()
	case 'h':
		s.prompt.list.PageDown()
	case 'l':
		s.prompt.list.PageUp()
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
	s.prompt.list, err = NewList(items, s.prompt.list.Size())
	s.prompt.list.SetCursor(idx)
	// return err
	return nil
}

// add or reset selected entry
func (s *Status) addReset() error {
	defer s.prompt.render()
	items, idx := s.prompt.list.Items()
	item := items[idx].(*git.StatusEntry)
	if item.Indexed() {
		return s.Repo.RemoveFromIndex(item)
	}
	return s.Repo.AddToIndex(item)
}

// open hunk stagin ui
func (s *Status) hunkStage() error {
	defer s.prompt.reader.Terminal.Out.Write([]byte(writer.HideCursor))
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
	return PopGenericCmd(s.Repo, fileStatArgs(entry))
}

func (s *Status) doCommit() error {
	defer s.prompt.reader.Terminal.Out.Write([]byte(writer.HideCursor))

	args := []string{"commit", "--edit", "--quiet"}
	err := PopGenericCmd(s.Repo, args)
	if err != nil {
		return err
	}
	if err := PopGenericCmd(s.Repo, lastCommitArgs(s.Repo)); err != nil {
		return err
	}
	return nil
}

func (s *Status) doCommitAmend() error {
	defer s.prompt.reader.Terminal.Out.Write([]byte(writer.HideCursor))

	args := []string{"commit", "--amend", "--quiet"}
	err := PopGenericCmd(s.Repo, args)
	if err != nil {
		return err
	}
	if err := PopGenericCmd(s.Repo, lastCommitArgs(s.Repo)); err != nil {
		return err
	}
	return nil
}
