package prompt

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/isacikgoz/gitin/term"
	git "github.com/isacikgoz/libgit2-api"
	"github.com/justincampbell/timeago"
)

// Log holds a list of items used to fill the terminal screen.
type Log struct {
	Repo *git.Repository

	prompt   *prompt
	selected *git.Commit
	oldState *promptState
}

// Start draws the screen with its list, initializing the cursor to the given position.
func (l *Log) Start(opts *Options) error {
	cs, err := l.Repo.Commits()
	if err != nil {
		return err
	}
	l.Repo.Branches()
	l.Repo.Tags()
	items := make([]Item, 0)
	for _, commit := range cs {
		items = append(items, commit)
	}
	list, err := NewList(items, opts.Size)
	if err != nil {
		return err
	}
	controls := make(map[string]string)
	controls["show diff"] = "d"
	controls["show stat"] = "s"
	controls["select"] = "enter"

	opts.SearchLabel = "Commits"

	l.prompt = create(opts,
		list,
		withOnKey(l.onKey),
		withSelection(l.onSelect),
		withInfo(l.logInfo),
	)
	l.prompt.controls = controls
	if err := l.prompt.Run(); err != nil {
		return err
	}
	return nil
}

// return true to terminate
func (l *Log) onSelect() bool {
	// s.showDiff()
	items, idx := l.prompt.list.Items()
	if idx == NotFound {
		return false
	}
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
		l.oldState = l.prompt.getState()
		list, err := NewList(newlist, 5)
		if err != nil {
			return false
		}
		l.prompt.setState(&promptState{
			list:       list,
			searchMode: false,
			searchStr:  "",
		})
		l.prompt.opts.SearchLabel = "Files"
	case *git.DiffDelta:
		l.showFileDiff()
	}
	return false
}

func (l *Log) onKey(key rune) bool {
	items, idx := l.prompt.list.Items()
	var item Item
	if idx != NotFound {
		item = items[idx]
	}
	switch item.(type) {
	case *git.Commit:
		switch key {
		case 's':
			l.showStat()
		case 'd':
			l.showDiff()
		case 'q':
			l.prompt.quit <- true
			return true
		}
	case *git.DiffDelta:
		switch key {
		case 'q':
			l.prompt.setState(l.oldState)
			l.prompt.opts.SearchLabel = "Commits"
		}
	}
	return false
}

func (l *Log) showDiff() error {
	items, idx := l.prompt.list.Items()
	if idx == NotFound {
		return fmt.Errorf("there is no item to show diff")
	}
	commit := items[idx].(*git.Commit)
	args := []string{"show", commit.Hash}
	return popGitCommand(l.Repo, args)
}

func (l *Log) showStat() error {
	items, idx := l.prompt.list.Items()
	if idx == NotFound {
		return fmt.Errorf("there is no item to show diff")
	}
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

func (l *Log) logInfo(item Item) [][]term.Cell {
	grid := make([][]term.Cell, 0)
	if item == nil {
		return grid
	}
	switch item.(type) {
	case *git.Commit:
		commit := item.(*git.Commit)
		cells := term.Cprint("Author ", color.Faint)
		cells = append(cells, term.Cprint(commit.Author.Name+" <"+commit.Author.Email+">", color.FgWhite)...)
		grid = append(grid, cells)
		cells = term.Cprint("When", color.Faint)
		cells = append(cells, term.Cprint("   "+timeago.FromTime(commit.Author.When), color.FgWhite)...)
		grid = append(grid, cells)
		grid = append(grid, commitRefs(l.Repo, commit))
		return grid
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
		var cells []term.Cell
		if adds > 1 {
			cells = term.Cprint(strconv.Itoa(adds-1), color.FgGreen)
			cells = append(cells, term.Cprint(" additions", color.Faint)...)
		}
		if dels > 1 {
			if len(cells) > 1 {
				cells = append(cells, term.Cell{Ch: ' '})
			}
			cells = append(cells, term.Cprint(strconv.Itoa(dels-1), color.FgRed)...)
			cells = append(cells, term.Cprint(" deletions", color.Faint)...)
		}
		if len(cells) > 1 {
			cells = append(cells, term.Cell{Ch: '.', Attr: []color.Attribute{color.Faint}})
		}
		grid = append(grid, cells)
	}
	return grid
}

func commitRefs(r *git.Repository, c *git.Commit) []term.Cell {
	var cells []term.Cell
	if refs, ok := r.RefMap[c.Hash]; ok {
		if len(refs) <= 0 {
			return cells
		}
		cells = term.Cprint("(", color.FgYellow)
		for _, ref := range refs {
			switch ref.Type() {
			case git.RefTypeHEAD:
				cells = append(cells, term.Cprint("HEAD -> ", color.FgCyan, color.Bold)...)
				cells = append(cells, term.Cprint(ref.String(), color.FgGreen, color.Bold)...)
				cells = append(cells, term.Cprint(", ", color.FgYellow)...)
			case git.RefTypeTag:
				cells = append(cells, term.Cprint("tag: ", color.FgYellow, color.Bold)...)
				cells = append(cells, term.Cprint(ref.String(), color.FgRed, color.Bold)...)
				cells = append(cells, term.Cprint(", ", color.FgYellow)...)
			case git.RefTypeBranch:
				cells = append(cells, term.Cprint(ref.String(), color.FgRed, color.Bold)...)
				cells = append(cells, term.Cprint(", ", color.FgYellow)...)
			}
		}
		cells = cells[:len(cells)-2]
		cells = append(cells, term.Cprint(")", color.FgYellow)...)
	}
	return cells
}
