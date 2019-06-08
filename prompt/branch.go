package prompt

import (
	"os/exec"

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
		str = append(str, faint.Sprint("Author")+" "+target.Author.Name+" <"+target.Author.Email+">")
		str = append(str, faint.Sprint("When")+"   "+timeago.FromTime(target.Author.When))
	}
	return str
}
