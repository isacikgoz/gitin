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

// branch holds a list of items used to fill the terminal screen.
type branch struct {
	repository *git.Repository
	prompt     *prompt.Prompt
}

// BranchPrompt draws the screen with its list, initializing the cursor to the given position.
func BranchPrompt(r *git.Repository, opts *prompt.Options) (*prompt.Prompt, error) {
	branches, err := r.Branches()
	if err != nil {
		return nil, fmt.Errorf("could not load branches: %v", err)
	}
	items := make([]prompt.Item, 0)
	for _, branch := range branches {
		items = append(items, branch)
	}
	list, err := prompt.NewList(items, opts.LineSize)
	if err != nil {
		return nil, fmt.Errorf("could not create list: %v", err)
	}
	controls := make(map[string]string)
	controls["delete branch"] = "d"
	controls["force delete"] = "D"
	controls["checkout"] = "enter"

	b := &branch{repository: r}
	b.prompt = prompt.Create("Branches", opts, list,
		prompt.WithKeyHandler(b.onKey),
		prompt.WithSelectionHandler(b.onSelect),
		prompt.WithItemRenderer(renderItem),
		prompt.WithInformation(b.branchInfo),
	)
	b.prompt.Controls = controls

	return b.prompt, nil
}

func (b *branch) onSelect() error {
	item, err := b.prompt.Selection()
	if err != nil {
		return nil
	}
	branch := item.(*git.Branch)
	args := []string{"checkout", branch.Name}
	cmd := exec.Command("git", args...)
	cmd.Dir = b.repository.Path()
	if err := cmd.Run(); err != nil {
		return nil // possibly dirty branch
	}
	b.prompt.Stop() // quit after selection
	return nil
}

func (b *branch) onKey(key rune) error {
	switch key {
	case 'd':
		b.deleteBranch("d")
	case 'D':
		b.deleteBranch("D")
	case 'q':
		b.prompt.Stop()
	}
	return nil
}

func (b *branch) branchInfo(item prompt.Item) [][]term.Cell {
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

func (b *branch) deleteBranch(mode string) error {
	item, err := b.prompt.Selection()
	if err != nil {
		return fmt.Errorf("could not delete branch: %v", err)
	}
	branch := item.(*git.Branch)
	cmd := exec.Command("git", "branch", "-"+mode, branch.Name)
	cmd.Dir = b.repository.Path()
	if err := cmd.Run(); err != nil {
		return err // possibly an unmerged branch
	}
	return b.reloadBranches()
}

// reloads the list
func (b *branch) reloadBranches() error {
	branches, err := b.repository.Branches()
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
