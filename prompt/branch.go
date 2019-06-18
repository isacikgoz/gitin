package prompt

import (
	"fmt"
	"os/exec"

	"github.com/fatih/color"
	"github.com/isacikgoz/gitin/term"
	git "github.com/isacikgoz/libgit2-api"
	"github.com/justincampbell/timeago"
)

// Branch holds a list of items used to fill the terminal screen.
type Branch struct {
	Repo   *git.Repository
	prompt *prompt
}

// Start draws the screen with its list, initializing the cursor to the given position.
func (b *Branch) Start(opts *Options) error {
	branches, err := b.Repo.Branches()
	if err != nil {
		return err
	}
	items := make([]Item, 0)
	for _, branch := range branches {
		items = append(items, branch)
	}
	list, err := NewList(items, opts.Size)
	if err != nil {
		return err
	}
	controls := make(map[string]string)
	controls["delete branch"] = "d"
	controls["force delete"] = "D"
	controls["checkout"] = "enter"

	opts.SearchLabel = "Branches"

	b.prompt = &prompt{
		list:      list,
		opts:      opts,
		selection: b.onSelect,
		keys:      b.onKey,
		info:      b.branchInfo,
		controls:  controls,
	}

	return b.prompt.start()
}

func (b *Branch) onSelect() bool {
	items, idx := b.prompt.list.Items()
	if idx == NotFound {
		return false
	}
	branch := items[idx].(*git.Branch)
	args := []string{"checkout", branch.Name}
	cmd := exec.Command("git", args...)
	cmd.Dir = b.Repo.Path()
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func (b *Branch) onKey(key rune) bool {
	switch key {
	case 'd':
		b.deleteBranch("d")
	case 'D':
		b.deleteBranch("D")
	case 'q':
		b.prompt.quit <- true
		return true
	}
	return false
}

func (b *Branch) branchInfo(item Item) [][]term.Cell {
	branch := item.(*git.Branch)
	target := branch.Target()
	grid := make([][]term.Cell, 0)
	if target != nil {
		cells := term.Cprint("Last commit was ", color.Faint)
		cells = append(cells, term.Cprint(timeago.FromTime(target.Author.When), color.FgBlue)...)
		grid = append(grid, cells)
		if branch.IsRemote() {
			return grid
		}
		grid = append(grid, branchInfo(branch, false)...)
	}
	return grid
}

func (b *Branch) deleteBranch(mode string) error {
	items, idx := b.prompt.list.Items()
	if idx == NotFound {
		return fmt.Errorf("there is no item to delete")
	}
	branch := items[idx].(*git.Branch)
	cmd := exec.Command("git", "branch", "-"+mode, branch.Name)
	cmd.Dir = b.Repo.Path()
	if err := cmd.Run(); err != nil {
		return err
	}
	return b.reloadBranches()
}

// reloads the list
func (b *Branch) reloadBranches() error {
	_, idx := b.prompt.list.Items()
	branches, err := b.Repo.Branches()
	if err != nil {
		return err
	}
	items := make([]Item, 0)
	for _, branch := range branches {
		items = append(items, branch)
	}
	b.prompt.list, err = NewList(items, b.prompt.list.size)
	b.prompt.list.SetCursor(idx)
	// return err
	return nil
}
