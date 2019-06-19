package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/isacikgoz/gitin/prompt"
	"github.com/isacikgoz/gitin/term"
	git "github.com/isacikgoz/libgit2-api"
	"github.com/justincampbell/timeago"
)

// log holds the repository struct and the prompt pointer. since log and prompt dependent,
// I found the best wau to associate them with this way
type log struct {
	repository *git.Repository
	prompt     *prompt.Prompt
	selected   *git.Commit
	oldState   *prompt.State
}

// LogPrompt draws the screen with its list, initializing the cursor to the given position.
func LogPrompt(r *git.Repository, opts *prompt.Options) (*prompt.Prompt, error) {
	cs, err := r.Commits()
	if err != nil {
		return nil, fmt.Errorf("could not load commits: %v", err)
	}
	r.Branches() // to find refs
	r.Tags()
	items := make([]prompt.Item, 0)
	for _, commit := range cs {
		items = append(items, commit)
	}
	list, err := prompt.NewList(items, opts.LineSize)
	if err != nil {
		return nil, fmt.Errorf("could not create list: %v", err)
	}
	controls := make(map[string]string)
	controls["show diff"] = "d"
	controls["show stat"] = "s"
	controls["select"] = "enter"

	l := &log{repository: r}
	l.prompt = prompt.Create("Commits", opts, list,
		prompt.WithKeyHandler(l.onKey),
		prompt.WithSelectionHandler(l.onSelect),
		prompt.WithItemRenderer(renderItem),
		prompt.WithInformation(l.logInfo),
	)
	l.prompt.Controls = controls

	return l.prompt, nil
}

// return true to terminate
func (l *log) onSelect() error {

	item, err := l.prompt.Selection()
	if err != nil {
		return nil
	}
	switch item.(type) {
	case *git.Commit:
		commit := item.(*git.Commit)
		l.selected = commit
		diff, err := commit.Diff()
		if err != nil {
			return nil
		}
		deltas := diff.Deltas()
		newlist := make([]prompt.Item, 0)
		for _, delta := range deltas {
			newlist = append(newlist, delta)
		}
		l.oldState = l.prompt.State()
		list, err := prompt.NewList(newlist, 5)
		if err != nil {
			return err
		}
		l.prompt.SetState(&prompt.State{
			List:        list,
			SearchMode:  false,
			SearchStr:   "",
			SearchLabel: "Files",
		})
		// l.prompt.opts.SearchLabel = "Files"
	case *git.DiffDelta:
		l.showFileDiff()
	}
	return nil
}

func (l *log) onKey(key rune) error {
	var item prompt.Item
	var err error
	item, err = l.prompt.Selection()
	if err != nil {
		return err
	}
	switch item.(type) {
	case *git.Commit:
		switch key {
		case 's':
			l.showStat()
		case 'd':
			l.showDiff()
		case 'q':
			l.prompt.Stop()
		}
	case *git.DiffDelta:
		switch key {
		case 'q':
			l.prompt.SetState(l.oldState)
		}
	}
	return nil
}

func (l *log) showDiff() error {
	item, err := l.prompt.Selection()
	if err != nil {
		return fmt.Errorf("there is no item to show diff")
	}
	commit := item.(*git.Commit)
	args := []string{"show", commit.Hash}
	return popGitCommand(l.repository, args)
}

func (l *log) showStat() error {
	item, err := l.prompt.Selection()
	if err != nil {
		return fmt.Errorf("there is no item to show diff")
	}
	commit := item.(*git.Commit)
	args := []string{"show", "--stat", commit.Hash}
	return popGitCommand(l.repository, args)
}

func (l *log) showFileDiff() error {
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
	item, err := l.prompt.Selection()
	if err != nil {
		return fmt.Errorf("there is no item to show diff")
	}
	dd := item.(*git.DiffDelta)
	args = append(args, dd.OldFile.Path)
	return popGitCommand(l.repository, args)
}

func (l *log) logInfo(item prompt.Item) [][]term.Cell {
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
		grid = append(grid, commitRefs(l.repository, commit))
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
