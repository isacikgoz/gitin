package git

import (
	"errors"
	"path/filepath"

	lib "github.com/libgit2/git2go"
)

// Repository is the wrapper and main interface to git repository
type Repository struct {
	essence *lib.Repository
	path    string

	RefMap map[string][]Ref
	Head   *Branch
}

// RefType defines the ref types
type RefType uint8

// These types are used for mapping references
const (
	RefTypeTag RefType = iota
	RefTypeBranch
	RefTypeHEAD
)

// Ref is the wrapper of lib.Ref
type Ref interface {
	Type() RefType
	Target() *Commit
	String() string
}

// Open load the repository from the filesystem
func Open(path string) (*Repository, error) {
	repo, realpath, err := initRepoFromPath(path)
	if err != nil {
		return nil, ErrCannotOpenRepo
	}
	r := &Repository{
		path:    realpath,
		essence: repo,
	}
	r.RefMap = make(map[string][]Ref)
	r.LoadHead()
	return r, nil
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

// LoadHead can be used to refresh HEAD ref
func (r *Repository) LoadHead() error {
	head, err := r.essence.Head()
	if err != nil {
		return err
	}
	branch, err := unpackRawBranch(r.essence, head.Branch())
	if err != nil {
		return err
	}
	obj, err := r.essence.RevparseSingle(branch.Hash)
	if err == nil && obj != nil {
		if commit, _ := obj.AsCommit(); commit != nil {
			branch.target = unpackRawCommit(r, commit)
		}
	}
	if err != nil {
		// a warning here
	}
	r.Head = branch
	return nil
}

// Path returns the filesystem location of the repository
func (r *Repository) Path() string {
	return r.path
}
