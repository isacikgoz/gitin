package prompt

import (
	git "github.com/isacikgoz/libgit2-api"
)

// Log holds a list of items used to fill the terminal screen.
type Log struct {
	Repo  *git.Repository
	Items []Item

	prompt   *prompt
	selected *git.Commit
	mainList *List
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
	items, idx := l.prompt.list.Items()
	item := items[idx]
	switch item.(type) {
	case *git.Commit:
		commit := item.(*git.Commit)
		l.selected = commit
		diff, err := commit.Diff()
		if err != nil {
			return false
		}
		deltas := diff.Deltas()
		newlist := make([]Item, 0)
		for _, delta := range deltas {
			newlist = append(newlist, delta)
		}
		l.mainList = l.prompt.list
		list, err := NewList(newlist, 5)
		if err != nil {
			return false
		}
		l.prompt.opts.SearchLabel = "Files"
		l.prompt.list = list
	case *git.DiffDelta:
		l.showFileDiff()
	}
	return false
}

func (l *Log) onKey(key rune) bool {
	switch key {
	case 'k':
		l.prompt.list.Prev()
	case 'j':
		l.prompt.list.Next()
	case 'h':
		l.prompt.list.PageDown()
	case 'l':
		l.prompt.list.PageUp()
	}

	items, idx := l.prompt.list.Items()
	item := items[idx]
	switch item.(type) {
	case *git.Commit:
		switch key {
		case 's':
			l.showStat()
		case 'd':
			l.showDiff()
		case 'q':
			return true
		}
	case *git.DiffDelta:
		switch key {
		case 'q':
			l.prompt.list = l.mainList
			l.prompt.opts.SearchLabel = "Commits"
		}
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

func (l *Log) showFileDiff() error {
	if l.selected == nil {
		return nil
	}
	var args []string
	pid, err := l.selected.ParentID()
	if err != nil {
		args = []string{"show", "--oneline", "--patch"}
	} else {
		args = []string{"diff", pid + ".." + l.selected.Hash}
	}
	items, idx := l.prompt.list.Items()
	dd := items[idx].(*git.DiffDelta)
	args = append(args, dd.OldFile.Path)
	return PopGenericCmd(l.Repo, args)
}
