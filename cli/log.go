package cli

import (
	"context"
	"fmt"
	"strconv"

	"github.com/fatih/color"
	"github.com/isacikgoz/gitin/git"
	"github.com/isacikgoz/gitin/prompt"
	"github.com/isacikgoz/gitin/term"
	"github.com/justincampbell/timeago"
)

// log holds the repository struct and the prompt pointer. since log and prompt dependent,
// I found the best wau to associate them with this way
type log struct {
	repository *git.Repository
	prompt     *prompt.Prompt
	refs       map[string][]git.Ref
	selected   *git.Commit
	oldState   *prompt.State
}

// LogPrompt configures a prompt to serve as a commit prompt. Commits are
// loaded in the background until ctx is cancelled.
func LogPrompt(ctx context.Context, r *git.Repository, opts *prompt.Options) (*prompt.Prompt, error) {
	l, err := newLog(ctx, r, opts)
	if err != nil {
		return nil, err
	}
	return l.prompt, nil
}

func newLog(ctx context.Context, r *git.Repository, opts *prompt.Options) (*log, error) {
	commits, wait := r.Commits(ctx)
	// git log keeps streaming commits while the refs are loaded
	refs, err := r.Refs()
	if err != nil {
		return nil, fmt.Errorf("could not load refs: %v", err)
	}

	items := make(chan interface{}, 1024)
	list, err := prompt.NewAsyncList(items, opts.LineSize)
	if err != nil {
		return nil, fmt.Errorf("could not create list: %v", err)
	}

	l := &log{repository: r, refs: refs}
	l.prompt = prompt.Create("Commits", opts, list,
		prompt.WithSelectionHandler(l.onSelect),
		prompt.WithItemRenderer(renderItem),
		prompt.WithInformation(l.logInfo),
	)
	if err := l.defineKeybindings(); err != nil {
		return nil, err
	}

	go func() {
		for c := range commits {
			items <- c
		}
		close(items)
		if err := wait(); err != nil && ctx.Err() == nil {
			l.prompt.Fail(fmt.Errorf("could not load commits: %v", err))
		}
	}()

	return l, nil
}

func (l *log) onSelect(item interface{}) error {
	switch item := item.(type) {
	case *git.Commit:
		l.selected = item
		diff, err := item.Diff()
		if err != nil {
			return err
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
		return popGitCommand(l.repository, fileDiffArgs(l.selected, item))
	}
	return nil
}

// fileDiffArgs returns the git command args showing the change of a file in
// a commit, compared to the first parent like the file list
func fileDiffArgs(c *git.Commit, dd *git.DiffDelta) []string {
	if pid, err := c.ParentID(); err == nil {
		return []string{"diff", pid, c.Hash, "--", dd.OldFile.Path}
	}
	return []string{"show", "--format=", c.Hash, "--", dd.OldFile.Path}
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
	switch item := item.(type) {
	case *git.Commit:
		cells := term.Cprint("Author ", color.Faint)
		cells = append(cells, term.Cprint(item.Author.Name+" <"+item.Author.Email+">", color.FgWhite)...)
		grid = append(grid, cells)
		cells = term.Cprint("When", color.Faint)
		cells = append(cells, term.Cprint("   "+timeago.FromTime(item.Author.When), color.FgWhite)...)
		grid = append(grid, cells)
		grid = append(grid, commitRefs(l.refs, item))
		return grid
	case *git.DiffDelta:
		if item.Binary {
			return append(grid, term.Cprint("Binary file.", color.Faint))
		}
		var cells []term.Cell
		if item.Additions > 0 {
			cells = term.Cprint(strconv.Itoa(item.Additions), color.FgGreen)
			cells = append(cells, term.Cprint(plural(item.Additions, " addition"), color.Faint)...)
		}
		if item.Deletions > 0 {
			if len(cells) > 1 {
				cells = append(cells, term.Cprint(", ", color.Faint)...)
			}
			cells = append(cells, term.Cprint(strconv.Itoa(item.Deletions), color.FgRed)...)
			cells = append(cells, term.Cprint(plural(item.Deletions, " deletion"), color.Faint)...)
		}
		if len(cells) > 1 {
			cells = append(cells, term.Cell{Ch: '.', Attr: []color.Attribute{color.Faint}})
		}
		grid = append(grid, cells)
	}
	return grid
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

func (l *log) defineKeybindings() error {
	keybindings := []*prompt.KeyBinding{
		{
			Key:     's',
			Display: "s",
			Desc:    "show stat",
			Handler: l.commitStat,
		},
		{
			Key:     'd',
			Display: "d",
			Desc:    "show diff",
			Handler: l.commitDiff,
		},
		{
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

func commitRefs(refMap map[string][]git.Ref, c *git.Commit) []term.Cell {
	var cells []term.Cell
	if refs, ok := refMap[c.Hash]; ok {
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
