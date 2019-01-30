package git

import (
	lib "gopkg.in/libgit2/git2go.v27"
)

// Repository is the main entity of the application.
type Repository struct {
	RepoID   string
	Name     string
	AbsPath  string
	repo     *lib.Repository
	Status   *Status
	Branch   *Branch
	Branches []*Branch
	Commits  []*Commit
	Remotes  []*Remote
	Tags     []*Tag
	Ahead    int
	Behind   int
}

type Remote struct {
	Name string
	URL  []string
}

func Open(path string) (*Repository, error) {
	r, err := lib.OpenRepository(path)
	if err != nil {
		return nil, err
	}
	repo := &Repository{
		RepoID:  "",
		Name:    "",
		AbsPath: path,
		repo:    r,
	}
	if err := repo.loadStatus(); err != nil {
		return nil, err
	}
	return repo, err
}

func (r *Repository) InitializeBranches() error {
	if err := r.loadBranches(); err != nil {
		return err
	}
	return nil
}

func (r *Repository) InitializeTags() error {
	if _, err := r.loadTags(); err != nil {
		return err
	}
	return nil
}

func (r *Repository) InitializeCommits(opts *CommitLoadOptions) error {
	if shallow, err := r.repo.IsShallow(); shallow || err != nil {
		commits, err := r.failOverShallow(opts)
		r.Commits = commits
		return err
	}
	head, err := r.repo.Head()
	if err != nil {
		return err
	}
	commits, err := r.loadCommits(head.Target(), nil, opts)
	if err != nil {
		return err
	}
	r.Commits = commits
	return nil
}
