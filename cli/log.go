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

// LogPrompt configures a prompt to serve as a commit prompt
func LogPrompt(r *git.Repository, opts *prompt.Options) (*prompt.Prompt, error) {
	cs, err := r.Commits()
	if err != nil {
		return nil, fmt.Errorf("could not load commits: %v", err)
	}
	r.Branches() // to find refs
	r.Tags()
	list, err := prompt.NewList(cs, opts.LineSize)
	if err != nil {
		return nil, fmt.Errorf("could not create list: %v", err)
	}

	l := &log{repository: r}
	l.prompt = prompt.Create("Commits", opts, list,
		prompt.WithSelectionHandler(l.onSelect),
		prompt.WithItemRenderer(renderItem),
		prompt.WithInformation(l.logInfo),
	)
	if err := l.defineKeybindings(); err != nil {
		return nil, err
	}

	return l.prompt, nil
}

// return true to terminate
func (l *log) onSelect(item interface{}) error {
	switch item.(type) {
	case *git.Commit:
		commit := item.(*git.Commit)
		l.selected = commit
		diff, err := commit.Diff()
		if err != nil {
			return nil
		}
		deltas := diff.Deltas()
		if len(deltas) <= 0 {
			return nil
		}

		l.oldState = l.prompt.State()
		list, err := prompt.NewList(deltas, 5)
		if err != nil {
			return err
		}
		l.prompt.SetState(&prompt.State{
			List:        list,
			SearchMode:  false,
			SearchStr:   "",
			SearchLabel: "Files",
		})
	case *git.DiffDelta:
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
		dd := item.(*git.DiffDelta)
		args = append(args, dd.OldFile.Path)
		if err := popGitCommand(l.repository, args); err != nil {
			//no err handling required here
		}
	}
	return nil
}

func (l *log) commitStat(item interface{}) error {
	commit, ok := item.(*git.Commit)
	if !ok {
		return nil
	}
	args := []string{"show", "--stat", commit.Hash}
	return popGitCommand(l.repository, args)
}

func (l *log) commitDiff(item interface{}) error {
	commit, ok := item.(*git.Commit)
	if !ok {
		return nil
	}
	args := []string{"show", commit.Hash}
	return popGitCommand(l.repository, args)
}

func (l *log) quit(item interface{}) error {
	switch item.(type) {
	case *git.Commit:
		l.prompt.Stop()
	case *git.DiffDelta:
		l.prompt.SetState(l.oldState)
	}
	return nil
}

func (l *log) logInfo(item interface{}) [][]term.Cell {
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

func (l *log) defineKeybindings() error {
	keybindings := []*prompt.KeyBinding{
		&prompt.KeyBinding{
			Key:     's',
			Display: "s",
			Desc:    "show stat",
			Handler: l.commitStat,
		},
		&prompt.KeyBinding{
			Key:     'd',
			Display: "d",
			Desc:    "show diff",
			Handler: l.commitDiff,
		},
		&prompt.KeyBinding{
			Key:     'q',
			Display: "q",
			Desc:    "quit",
			Handler: l.quit,
		},
	}
	for _, kb := range keybindings {
		if err := l.prompt.AddKeyBinding(kb); err != nil {
			return err
		}
	}
	return nil
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
