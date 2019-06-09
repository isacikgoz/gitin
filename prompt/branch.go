package prompt

import (
	"os/exec"
	"strconv"

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
	opts.SearchLabel = "Branches"
	b.prompt = &prompt{
		repo:      b.Repo,
		list:      list,
		opts:      opts,
		layout:    branch,
		selection: b.onSelect,
		keys:      b.onKey,
		info:      b.branchInfo,
	}

	return b.prompt.start()
}

func (b *Branch) onSelect() bool {
	items, idx := b.prompt.list.Items()
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
	case 'k':
		b.prompt.list.Prev()
	case 'j':
		b.prompt.list.Next()
	case 'h':
		b.prompt.list.PageDown()
	case 'l':
		b.prompt.list.PageUp()
	case 'd':
		b.deleteBranch("d")
	case 'D':
		b.deleteBranch("D")
	case 'q':
		return true
	}
	return false
}

func (b *Branch) branchInfo(item Item) []string {
	branch := item.(*git.Branch)
	target := branch.Target()
	str := make([]string, 0)
	if target != nil {
		str = append(str, faint.Sprint("Last commit was")+" "+timeago.FromTime(target.Author.When))
		if branch.IsRemote() {
			return str
		}
		if branch.Upstream == nil {
			str = append(str, faint.Sprint("This branch is not tracking a remote branch."))
			return str
		}
		pl := branch.Behind
		ps := branch.Ahead

		if ps == 0 && pl == 0 {
			str = append(str, faint.Sprint("This branch is up to date with ")+cyan.Sprint(branch.Upstream.Name)+faint.Sprint("."))
		} else {
			if ps > 0 && pl > 0 {
				str = append(str, faint.Sprint("This branch and ")+cyan.Sprint(branch.Upstream.Name)+faint.Sprint(" have diverged,"))
				str = append(str, faint.Sprint("and have ")+yellow.Sprint(strconv.Itoa(ps))+faint.Sprint(" and ")+yellow.Sprint(strconv.Itoa(pl))+faint.Sprint(" different commits each, respectively."))
			} else if pl > 0 && ps == 0 {
				str = append(str, faint.Sprint("This branch is behind ")+cyan.Sprint(branch.Upstream.Name)+faint.Sprint(" by ")+yellow.Sprint(strconv.Itoa(pl))+faint.Sprint(" commit(s)."))
			} else if ps > 0 && pl == 0 {
				str = append(str, faint.Sprint("This branch is ahead of ")+cyan.Sprint(branch.Upstream.Name)+faint.Sprint(" by ")+yellow.Sprint(strconv.Itoa(ps))+faint.Sprint(" commit(s)."))
			}
		}
	}
	return str
}

func (b *Branch) deleteBranch(mode string) error {
	items, idx := b.prompt.list.Items()
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
