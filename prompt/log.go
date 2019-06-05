package prompt

import git "github.com/isacikgoz/libgit2-api"

// Log holds a list of items used to fill the terminal screen.
type Log struct {
	Repo  *git.Repository
	Items []Item

	prompt *prompt
}

// Start draws the screen with its list, initializing the cursor to the given position.
func (l *Log) Start(opts *Options) error {
	list, err := NewList(l.Items, opts.Size)
	if err != nil {
		return err
	}
	l.prompt = &prompt{
		repo:      l.Repo,
		list:      list,
		opts:      opts,
		layout:    log,
		keys:      l.onKey,
		selection: l.onSelect,
	}

	return l.prompt.start()
}

// return true to terminate
func (l *Log) onSelect() bool {
	// s.showDiff()
	return false
}

func (l *Log) onKey(key rune) bool {
	// var reqReload bool

	switch key {
	case 'k':
		l.prompt.list.Prev()
	case 'j':
		l.prompt.list.Next()
	case 'h':
		l.prompt.list.PageDown()
	case 'l':
		l.prompt.list.PageUp()
	case 's':
		l.showStat()
	case 'd':
		l.showDiff()
	case 'q':
		return true
	}
	return false
}

func (l *Log) showDiff() error {
	items, idx := l.prompt.list.Items()
	commit := items[idx].(*git.Commit)
	args := []string{"show", commit.Hash}
	return PopGenericCmd(l.Repo, args)
}

func (l *Log) showStat() error {
	items, idx := l.prompt.list.Items()
	commit := items[idx].(*git.Commit)
	args := []string{"show", "--stat", commit.Hash}
	return PopGenericCmd(l.Repo, args)
}
