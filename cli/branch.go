package cli

import (
	"fmt"
	"os/exec"

	"github.com/fatih/color"
	"github.com/isacikgoz/gitin/prompt"
	"github.com/isacikgoz/gitin/term"
	git "github.com/isacikgoz/libgit2-api"
	"github.com/justincampbell/timeago"
)

// Branch holds a list of items used to fill the terminal screen.
type Branch struct {
	Repo   *git.Repository
	prompt *prompt.Prompt
}

// BranchPrompt draws the screen with its list, initializing the cursor to the given position.
func BranchPrompt(r *git.Repository, opts *prompt.Options) error {
	branches, err := r.Branches()
	if err != nil {
		return err
	}
	items := make([]prompt.Item, 0)
	for _, branch := range branches {
		items = append(items, branch)
	}
	list, err := prompt.NewList(items, opts.Size)
	if err != nil {
		return err
	}
	controls := make(map[string]string)
	controls["delete branch"] = "d"
	controls["force delete"] = "D"
	controls["checkout"] = "enter"

	opts.SearchLabel = "Branches"
	b := &Branch{Repo: r}
	b.prompt = prompt.Create(opts,
		list,
		prompt.WithKeyHandler(b.onKey),
		prompt.WithSelectionHandler(b.onSelect),
		prompt.WithItemRenderer(renderItem),
		prompt.WithInformation(b.branchInfo),
	)
	b.prompt.Controls = controls
	if err := b.prompt.Run(); err != nil {
		return err
	}
	return nil
}

func (b *Branch) onSelect() bool {
	item, err := b.prompt.Selection()
	if err != nil {
		return false
	}
	branch := item.(*git.Branch)
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
		b.prompt.Stop()
		return true
	}
	return false
}

func (b *Branch) branchInfo(item prompt.Item) [][]term.Cell {
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
	item, err := b.prompt.Selection()
	if err != nil {
		return fmt.Errorf("could not delete branch: %v", err)
	}
	branch := item.(*git.Branch)
	cmd := exec.Command("git", "branch", "-"+mode, branch.Name)
	cmd.Dir = b.Repo.Path()
	if err := cmd.Run(); err != nil {
		return err
	}
	return b.reloadBranches()
}

// reloads the list
func (b *Branch) reloadBranches() error {
	branches, err := b.Repo.Branches()
	if err != nil {
		return err
	}
	items := make([]prompt.Item, 0)
	for _, branch := range branches {
		items = append(items, branch)
	}
	state := b.prompt.State()
	list, err := prompt.NewList(items, state.ListSize)
	if err != nil {
		return fmt.Errorf("could not reload branches: %v", err)
	}
	state.List = list
	b.prompt.SetState(state)
	// return err
	return nil
}
