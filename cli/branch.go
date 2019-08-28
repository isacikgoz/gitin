package cli

import (
	"fmt"
	"os/exec"

	"github.com/fatih/color"
	"github.com/isacikgoz/gitin/prompt"
	"github.com/isacikgoz/gitin/term"
	"github.com/isacikgoz/gitin/git"
	"github.com/justincampbell/timeago"
)

// branch holds a list of items used to fill the terminal screen.
type branch struct {
	repository *git.Repository
	prompt     *prompt.Prompt
}

// BranchPrompt configures a prompt to serve as a branch prompt
func BranchPrompt(r *git.Repository, opts *prompt.Options) (*prompt.Prompt, error) {
	branches, err := r.Branches()
	if err != nil {
		return nil, fmt.Errorf("could not load branches: %v", err)
	}
	list, err := prompt.NewList(branches, opts.LineSize)
	if err != nil {
		return nil, fmt.Errorf("could not create list: %v", err)
	}

	b := &branch{repository: r}
	b.prompt = prompt.Create("Branches", opts, list,
		prompt.WithSelectionHandler(b.onSelect),
		prompt.WithItemRenderer(renderItem),
		prompt.WithInformation(b.branchInfo),
	)
	b.defineKeyBindings()

	return b.prompt, nil
}

func (b *branch) onSelect(item interface{}) error {
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

func (b *branch) defineKeyBindings() error {
	keybindings := []*prompt.KeyBinding{
		&prompt.KeyBinding{
			Key:     'd',
			Display: "d",
			Desc:    "delete branch",
			Handler: b.deleteBranch,
		},
		&prompt.KeyBinding{
			Key:     'D',
			Display: "D",
			Desc:    "force delete branch",
			Handler: b.forceDeleteBranch,
		},
		&prompt.KeyBinding{
			Key:     'q',
			Display: "q",
			Desc:    "quit",
			Handler: b.quit,
		},
	}
	for _, kb := range keybindings {
		if err := b.prompt.AddKeyBinding(kb); err != nil {
			return err
		}
	}
	return nil
}

func (b *branch) branchInfo(item interface{}) [][]term.Cell {
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

func (b *branch) deleteBranch(item interface{}) error {
	return b.bareDelete(item, "d")
}

func (b *branch) forceDeleteBranch(item interface{}) error {
	return b.bareDelete(item, "D")
}

func (b *branch) bareDelete(item interface{}, mode string) error {
	branch := item.(*git.Branch)
	cmd := exec.Command("git", "branch", "-"+mode, branch.Name)
	cmd.Dir = b.repository.Path()
	if err := cmd.Run(); err != nil {
		return nil // possibly an unmerged branch, just ignore it
	}
	return b.reloadBranches()
}

func (b *branch) quit(item interface{}) error {
	b.prompt.Stop()
	return nil
}

// reloads the list
func (b *branch) reloadBranches() error {
	branches, err := b.repository.Branches()
	if err != nil {
		return err
	}
	state := b.prompt.State()
	list, err := prompt.NewList(branches, state.ListSize)
	if err != nil {
		return fmt.Errorf("could not reload branches: %v", err)
	}
	state.List = list
	b.prompt.SetState(state)
	// return err
	return nil
}
