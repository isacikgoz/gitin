package prompt

import (
	"os"
	"sync"

	"github.com/isacikgoz/fig/git"

	"github.com/isacikgoz/fig/prompt/list"
	"github.com/isacikgoz/fig/prompt/screenbuf"

	"github.com/isacikgoz/gia/editor"

	"github.com/isacikgoz/sig/keys"
	"github.com/isacikgoz/sig/reader"

	log "github.com/sirupsen/logrus"
)

const (
	lines = 5
)

// Status holds a list of items used to fill the terminal screen.
type Status struct {
	Repo *git.Repository

	Items       []git.FuzzItem
	list        *list.List
	reader      *reader.RuneReader
	writeBuffer *screenbuf.ScreenBuf
	mx          *sync.RWMutex
}

// Start draws the screen with its list, initializing the cursor to the given position.
func (s *Status) Start(cursorPos, scroll int) error {
	l, err := list.New(s.Items, lines)
	if err != nil {
		log.Fatal(err)

		return err
	}
	s.list = l
	var mx sync.RWMutex
	s.mx = &mx

	term := reader.Terminal{
		In:  os.Stdin,
		Out: os.Stdout,
	}
	s.reader = reader.NewRuneReader(term)
	s.writeBuffer = screenbuf.New(term.Out)

	s.list.SetCursor(cursorPos)
	s.list.SetStart(scroll)

	return s.innerRun(cursorPos, scroll)
}

// this is the main loop for reading input
func (s *Status) innerRun(cursorPos, scroll int) error {
	var err error

	// disable echo
	s.reader.SetTermMode()
	defer s.reader.RestoreTermMode()

	// disable linewrap
	s.reader.Terminal.Out.Write([]byte(hideCursor))
	defer s.reader.Terminal.Out.Write([]byte(showCursor))

	// start with first render
	s.render()

	// start waiting for input
	for {
		items, _ := s.list.Items()
		if len(items) <= 0 && s.Repo.Head != nil {
			defer func() {
				for _, line := range branchClean(s.Repo.Head) {
					s.writeBuffer.Write([]byte(line))
				}
				s.writeBuffer.Flush()
			}()
			err = nil
			break
		}
		r, _, err := s.reader.ReadRune()
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
	}
	// reset cursor position and remove buffer
	s.writeBuffer.Reset()
	s.writeBuffer.ClearScreen()
	return err
}

// assignKey is called on every keypress.
func (s *Status) assignKey(key rune) bool {
	var reqReload bool
	switch key {
	case keys.Enter, '\n':
		s.showDiff()
	case keys.ArrowUp:
		s.list.Prev()
	case keys.ArrowDown:
		s.list.Next()
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
		s.addAll()
	case 'r':
		reqReload = true
		s.resetAll()
	case 'q':
		return true
	default:
	}
	if reqReload {
		s.reloadStatus()
	}
	s.render()
	return false
}

// render function draws screen's list to terminal
func (s *Status) render() {
	// make terminal not line wrap
	s.reader.Terminal.Out.Write([]byte(lineWrapOff))
	defer s.reader.Terminal.Out.Write([]byte(lineWrapOn))

	// lock screen mutex
	s.mx.Lock()
	defer s.mx.Unlock()

	items, idx := s.list.Items()

	if len(items) <= 0 && s.Repo.Head != nil {
		for _, line := range branchClean(s.Repo.Head) {
			s.writeBuffer.Write([]byte(line))
		}
		s.writeBuffer.Flush()
		return
	}
	s.writeBuffer.Write([]byte(faint.Sprint("Files:")))

	// print each entry in the list
	var output []byte
	for i := range items {
		if i == idx {
			output = []byte(cyan.Sprint(">") + renderLine(items[i], nil))
		} else {
			output = []byte(" " + renderLine(items[i], nil))
		}
		s.writeBuffer.Write(output)
	}

	// print repository status
	s.writeBuffer.Write([]byte(""))
	for _, line := range branchInfo(s.Repo.Head) {
		s.writeBuffer.Write([]byte(line))
	}

	// finally, discharge to terminal
	s.writeBuffer.Flush()
}

// reloads the list
func (s *Status) reloadStatus() error {
	_, idx := s.list.Items()
	items, err := s.Repo.ReloadStatusEntries()
	if err != nil {
		return err
	}
	s.list, err = list.New(items, s.list.Size())
	s.list.SetCursor(idx)
	return err
}

// add or reset selected entry
func (s *Status) addReset() error {
	defer s.render()
	items, idx := s.list.Items()
	item := items[idx].(*git.StatusEntry)
	if item.Indexed() {
		return s.Repo.ResetEntry(item)
	}
	return s.Repo.AddEntry(item)
}

// open hunk stagin ui
func (s *Status) hunkStage() error {
	defer s.reader.Terminal.Out.Write([]byte(hideCursor))
	items, idx := s.list.Items()
	entry := items[idx].(*git.StatusEntry)
	file, err := git.GenerateDiffFile(s.Repo, entry)
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
			if err := git.ApplyPatchCmd(s.Repo, entry, patch); err != nil {
				return err
			}
		}
	} else {
		log.Warn(err.Error())
	}
	return nil
}

// pop git diff
func (s *Status) showDiff() error {
	items, idx := s.list.Items()
	entry := items[idx].(*git.StatusEntry)
	return git.PopGenericCmd(s.Repo, entry.FileStatArgs())
}

func (s *Status) doCommit() error {
	defer s.reader.Terminal.Out.Write([]byte(hideCursor))

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
	defer s.reader.Terminal.Out.Write([]byte(hideCursor))

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

func (s *Status) addAll() error {
	return s.Repo.AddAll()
}

func (s *Status) resetAll() error {
	return s.Repo.ResetAll()
}
