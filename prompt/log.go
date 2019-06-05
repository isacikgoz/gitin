package prompt

import git "github.com/isacikgoz/libgit2-api"

// Log holds a list of items used to fill the terminal screen.
type Log struct {
	Repo  *git.Repository
	Items []Item

	prompt *prompt
}

// Start draws the screen with its list, initializing the cursor to the given position.
func (s *Log) Start(opts *Options) error {
	l, err := NewList(s.Items, opts.Size)
	if err != nil {
		return err
	}
	s.prompt = &prompt{
		repo:      s.Repo,
		list:      l,
		opts:      opts,
		layout:    log,
		keys:      s.onKey,
		selection: s.onSelect,
	}

	return s.prompt.start()
}

// return true to terminate
func (s *Log) onSelect() bool {
	// s.showDiff()
	return false
}

func (s *Log) onKey(key rune) bool {
	// var reqReload bool

	switch key {
	case 'k':
		s.prompt.list.Prev()
	case 'j':
		s.prompt.list.Next()
	case 'h':
		s.prompt.list.PageDown()
	case 'l':
		s.prompt.list.PageUp()
		// case keys.Space:
		// 	reqReload = true
		// 	s.addReset()
		// case 'p':
		// 	reqReload = true
		// 	s.hunkStage()
		// case 'c':
		// 	reqReload = true
		// 	s.doCommit()
		// case 'm':
		// 	reqReload = true
		// 	s.doCommitAmend()
		// case 'a':
		// 	reqReload = true
		// 	// TODO: check for errors
		// 	addAll(s.Repo)
		// case 'r':
		// 	reqReload = true
		// 	resetAll(s.Repo)
		// case 'q':
		// 	return true
		// default:
		// }
		// if reqReload {
		// 	s.reloadStatus()
	}
	return false
}
