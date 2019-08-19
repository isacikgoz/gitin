package git

import (
	"fmt"
	"strings"
	"time"

	lib "github.com/libgit2/git2go"
)

// Commit is the wrapper of actual lib.Commit object
type Commit struct {
	essence *lib.Commit
	owner   *Repository

	Author  *Signature
	Message string
	Summary string
	Hash    string
}

// Signature is the person who signs a commit
type Signature struct {
	Name  string
	Email string
	When  time.Time
}

func (s *Signature) toNewLibSignature() *lib.Signature {
	return &lib.Signature{
		Name:  s.Name,
		Email: s.Email,
		When:  s.When,
	}
}

// Commits returns all of the commits of the repository
func (r *Repository) Commits() ([]*Commit, error) {
	head, err := r.essence.Head()
	if err != nil {
		return nil, err
	}
	walk, err := r.essence.Walk()
	if err != nil {
		return nil, err
	}
	if err := walk.Push(head.Target()); err != nil {
		return nil, err
	}
	buffer := make([]*Commit, 0)
	defer walk.Free()
	err = walk.Iterate(func(commit *lib.Commit) bool {

		c := unpackRawCommit(r, commit)

		buffer = append(buffer, c)
		return true
	})
	return buffer, err
}

func unpackRawCommit(repo *Repository, raw *lib.Commit) *Commit {
	oid := raw.AsObject().Id()

	hash := oid.String()
	author := &Signature{
		Name:  raw.Author().Name,
		Email: raw.Author().Email,
		When:  raw.Author().When,
	}
	sum := raw.Summary()
	msg := raw.Message()

	c := &Commit{
		essence: raw,
		owner:   repo,
		Hash:    hash,
		Author:  author,
		Message: msg,
		Summary: sum,
	}
	return c
}

// Commit adds a new commit onject to repository
// warning: this function does not check if the changes are indexed
func (r *Repository) Commit(message string, author ...*Signature) (*Commit, error) {
	repo := r.essence
	head, err := repo.Head()
	if err != nil {
		return nil, err
	}
	defer head.Free()
	parent, err := repo.LookupCommit(head.Target())
	if err != nil {
		return nil, err
	}
	defer parent.Free()
	index, err := repo.Index()
	if err != nil {
		return nil, err
	}
	defer index.Free()
	treeid, err := index.WriteTree()
	if err != nil {
		return nil, err
	}
	tree, err := repo.LookupTree(treeid)
	if err != nil {
		return nil, err
	}
	defer tree.Free()
	oid, err := repo.CreateCommit("HEAD", author[0].toNewLibSignature(), author[0].toNewLibSignature(), message, tree, parent)
	if err != nil {
		return nil, err
	}
	commit, err := repo.LookupCommit(oid)
	if err != nil {
		return nil, err
	}
	return unpackRawCommit(r, commit), nil
}

func (c *Commit) String() string {
	return c.Summary
}

// Amend updates the commit and returns NEW commit pointer
func (c *Commit) Amend(message string, author ...*Signature) (*Commit, error) {
	repo := c.owner.essence
	index, err := repo.Index()
	if err != nil {
		return nil, err
	}
	defer index.Free()
	treeid, err := index.WriteTree()
	if err != nil {
		return nil, err
	}
	tree, err := repo.LookupTree(treeid)
	if err != nil {
		return nil, err
	}
	defer tree.Free()
	oid, err := c.essence.Amend("HEAD", author[0].toNewLibSignature(), author[0].toNewLibSignature(), message, tree)
	if err != nil {
		return nil, err
	}
	commit, err := repo.LookupCommit(oid)
	if err != nil {
		return nil, err
	}
	return &Commit{
		essence: commit,
		owner:   c.owner,
	}, nil
}

// Diff has similar behavior to "git diff <commit>"
func (c *Commit) Diff() (*Diff, error) {
	// if c.essence.ParentCount() > 1 {
	// 	return nil, errors.New("commit has multiple parents")
	// }

	cTree, err := c.essence.Tree()
	if err != nil {
		return nil, err
	}
	defer cTree.Free()
	var pTree *lib.Tree
	if c.essence.ParentCount() > 0 {
		if pTree, err = c.essence.Parent(0).Tree(); err != nil {
			return nil, err
		}
		defer pTree.Free()
	}

	opt, err := lib.DefaultDiffOptions()
	if err != nil {
		return nil, err
	}

	diff, err := c.owner.essence.DiffTreeToTree(pTree, cTree, &opt)
	if err != nil {
		return nil, err
	}
	defer diff.Free()

	stats, err := diff.Stats()
	if err != nil {
		return nil, err
	}

	statsText, err := stats.String(lib.DiffStatsFull, 80)
	if err != nil {
		return nil, err
	}
	ddeltas := make([]*DiffDelta, 0)
	patchs := make([]string, 0)
	deltas, err := diff.NumDeltas()
	if err != nil {
		return nil, err
	}

	var patch *lib.Patch
	var patchtext string

	for i := 0; i < deltas; i++ {
		if patch, err = diff.Patch(i); err != nil {
			continue
		}
		var dd lib.DiffDelta
		if dd, err = diff.GetDelta(i); err != nil {
			continue
		}
		d := &DiffDelta{
			Status: DeltaStatus(dd.Status),
			NewFile: &DiffFile{
				Path: dd.NewFile.Path,
				Hash: dd.NewFile.Oid.String(),
			},
			OldFile: &DiffFile{
				Path: dd.OldFile.Path,
				Hash: dd.OldFile.Oid.String(),
			},
			Commit: c,
		}

		if patchtext, err = patch.String(); err != nil {
			continue
		}
		d.Patch = patchtext

		ddeltas = append(ddeltas, d)
		patchs = append(patchs, patchtext)

		if err := patch.Free(); err != nil {
			return nil, err
		}
	}

	d := &Diff{
		deltas: ddeltas,
		stats:  strings.Split(statsText, "\n"),
		patchs: patchs,
	}
	return d, nil
}

// ParentID returns the commits parent hash.
func (c *Commit) ParentID() (string, error) {
	if c.essence.Parent(0) == nil {
		return "", fmt.Errorf("%s", "commit does not have parents")
	}
	return c.essence.Parent(0).AsObject().Id().String(), nil
}
