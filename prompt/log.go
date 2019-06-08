package prompt

import (
	"fmt"
	"strings"

	git "github.com/isacikgoz/libgit2-api"
	"github.com/justincampbell/timeago"
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
		info:      l.logInfo,
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
	return popGitCommand(l.Repo, args)
}

func (l *Log) showStat() error {
	items, idx := l.prompt.list.Items()
	commit := items[idx].(*git.Commit)
	args := []string{"show", "--stat", commit.Hash}
	return popGitCommand(l.Repo, args)
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
	return popGitCommand(l.Repo, args)
}

func (l *Log) logInfo(item Item) []string {
	str := make([]string, 0)
	if item == nil {
		return str
	}
	switch item.(type) {
	case *git.Commit:
		commit := item.(*git.Commit)
		str = append(str, faint.Sprint("Author")+" "+commit.Author.Name+" <"+commit.Author.Email+">")
		str = append(str, faint.Sprint("When")+"   "+timeago.FromTime(commit.Author.When))
		return str
	case *git.DiffDelta:
		dd := item.(*git.DiffDelta)
		var adds, dels int
		for _, line := range strings.Split(dd.Patch, "\n") {
			if len(line) > 0 {
				switch rn := line[0]; rn {
				case '+':
					adds++
				case '-':
					dels++
				}
			}
		}
		var infoLine string
		if adds > 1 {
			infoLine = fmt.Sprintf("%s %s", green.Sprintf("%d", adds-1), faint.Sprint("additions"))
		}
		if dels > 1 {
			if len(infoLine) > 1 {
				infoLine = infoLine + " "
			}
			infoLine = infoLine + fmt.Sprintf("%s %s", red.Sprintf("%d", dels-1), faint.Sprint("deletions"))
		}
		if len(infoLine) > 1 {
			infoLine = infoLine + faint.Sprint(".")
		}
		str = append(str, infoLine)
	}
	return str
}
