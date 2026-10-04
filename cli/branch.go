package cli

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/isacikgoz/gitin/git"
	"github.com/isacikgoz/gitin/prompt"
	"github.com/isacikgoz/gitin/term"
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
	if err := b.defineKeyBindings(); err != nil {
		return nil, err
	}

	return b.prompt, nil
}

func (b *branch) onSelect(item interface{}) error {
	branch := item.(*git.Branch)
	if err := runGitCommand(b.repository, []string{"checkout", branch.Name, "--"}); err != nil {
		return err // e.g. local changes would be overwritten
	}
	b.prompt.Stop() // quit after selection
	return nil
}

func (b *branch) defineKeyBindings() error {
	keybindings := []*prompt.KeyBinding{
		{
			Key:     'd',
			Display: "d",
			Desc:    "delete branch",
			Handler: b.deleteBranch,
		},
		{
			Key:     'D',
			Display: "D",
			Desc:    "force delete branch",
			Handler: b.forceDeleteBranch,
		},
		{
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
	grid := make([][]term.Cell, 0)
	if branch.When.IsZero() {
		return grid
	}
	cells := term.Cprint("Last commit was ", color.Faint)
	cells = append(cells, term.Cprint(timeago.FromTime(branch.When), color.FgBlue)...)
	grid = append(grid, cells)
	if branch.IsRemote() {
		return grid
	}
	return append(grid, branchInfo(branch, false)...)
}

func (b *branch) deleteBranch(item interface{}) error {
	return b.bareDelete(item, "d")
}

func (b *branch) forceDeleteBranch(item interface{}) error {
	return b.bareDelete(item, "D")
}

func (b *branch) bareDelete(item interface{}, mode string) error {
	branch := item.(*git.Branch)
	args := []string{"branch", "-" + mode, branch.Name}
	if branch.IsRemote() {
		args = []string{"branch", "-" + mode, "--remotes", branch.Name}
	}
	if err := runGitCommand(b.repository, args); err != nil {
		return err // e.g. the branch is not fully merged
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
	return nil
}
