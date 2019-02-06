package git

import (
	"errors"
	"path/filepath"

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

// Remote is to communicate with the outside world. fetch, pull or push operations
// are targetted to specific remotes
type Remote struct {
	Name string
	URL  []string
}

// Open the repository from given path and return Repository pointer
func Open(path string) (*Repository, error) {
	r, absPath, err := initRepoFromPath(path)
	if err != nil {
		return nil, err
	}
	repo := &Repository{
		RepoID:  "",
		Name:    "",
		AbsPath: absPath,
		repo:    r,
	}
	if err := repo.loadStatus(); err != nil {
		return nil, err
	}
	return repo, err
}

// InitializeBranches loads the branches
func (r *Repository) InitializeBranches() error {
	if err := r.loadBranches(); err != nil {
		return err
	}
	return nil
}

// InitializeStatus loads the files of working dir
func (r *Repository) InitializeStatus() error {
	if err := r.loadStatus(); err != nil {
		return err
	}
	return nil
}

// InitializeTags loads tags
func (r *Repository) InitializeTags() error {
	if _, err := r.loadTags(); err != nil {
		return err
	}
	return nil
}

// InitializeCommits loads all commits from current HEAD
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

func initRepoFromPath(path string) (*lib.Repository, string, error) {
	walk := path
	for {
		r, err := lib.OpenRepository(walk)
		if err == nil {
			return r, walk, err
		}
		walk = filepath.Dir(walk)
		if walk == "/" {
			break
		}
	}
	return nil, walk, errors.New("cannot load a git repository from " + path)
}
