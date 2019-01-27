package git

import (
	// "errors"
	"strings"
	"time"

	"github.com/justincampbell/timeago"

	log "github.com/sirupsen/logrus"
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
	Ahead    int
	Behind   int
}

type Branch struct {
	Name     string
	FullName string
	Hash     string
	Upstream *Branch
	Ahead    []*Commit
	Behind   []*Commit
	Clean    bool
	isRemote bool
}

type Remote struct {
	Name string
	URL  []string
}

type Commit struct {
	commit  *lib.Commit
	Hash    string
	Author  *Contributor
	Message string
	Summary string
	Type    CommitType
}

// CommitType is the Type of the commit; it can be local or remote (upstream diff)
type CommitType string

const (
	// LocalCommit is the commit that not pushed to remote branch
	LocalCommit CommitType = "local"
	// EvenCommit is the commit that recorded locally
	EvenCommit CommitType = "even"
	// RemoteCommit is the commit that not merged to local branch
	RemoteCommit CommitType = "remote"
)

// Contributor is the person
type Contributor struct {
	Name  string
	Email string
	When  time.Time
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

	if err := repo.loadBranches(); err != nil {
		return nil, err
	}

	if err := repo.loadCommits(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *Repository) loadBranches() error {
	bs := make([]*Branch, 0)
	branchIter, err := r.repo.NewBranchIterator(lib.BranchAll)
	if err != nil {
		return err
	}
	defer branchIter.Free()

	err = branchIter.ForEach(func(branch *lib.Branch, branchType lib.BranchType) error {

		name, err := branch.Name()
		if err != nil {
			return err
		}
		fullname := branch.Reference.Name()

		rawOid := branch.Target()

		if rawOid == nil {
			ref, err := branch.Resolve()
			if err != nil {
				return err
			}

			rawOid = ref.Target()
		}

		hash := rawOid.String()
		isRemote := branch.IsRemote()
		var upstream *Branch
		var aheads, behinds []*Commit
		if !isRemote {
			us, err := branch.Upstream()
			if err != nil || us == nil {
				log.Warn("upstream not found")
			} else {
				upstream = &Branch{
					Name:     strings.Replace(us.Name(), "refs/remotes/", "", 1),
					FullName: us.Name(),
					Hash:     us.Target().String(),
					isRemote: true,
				}
				a, b, err := r.repo.AheadBehind(branch.Target(), us.Target())
				if err == nil {
					aheads = make([]*Commit, a)
					behinds = make([]*Commit, b)
				}
			}
		}
		b := &Branch{
			Name:     name,
			FullName: fullname,
			Hash:     hash,
			isRemote: isRemote,
			Upstream: upstream,
			Ahead:    aheads,
			Behind:   behinds,
		}
		bs = append(bs, b)
		return nil
	})
	r.Branches = bs
	head, err := r.repo.Head()
	if err != nil {
		return err
	}
	for _, b := range r.Branches {
		if b.isRemote {
			continue
		}
		if head.Target().String() == b.Hash {
			r.Branch = b
		}
	}
	return err
}

func (r *Repository) loadCommits() error {
	cs := make([]*Commit, 0)

	head, err := r.repo.Head()
	if err != nil {
		return err
	}

	walk, err := r.repo.Walk()
	if err != nil {
		return err
	}
	var currentbranch *Branch
	for _, b := range r.Branches {
		if head.Target().String() == b.Hash {
			currentbranch = b
		}
	}
	if currentbranch != nil && currentbranch.Upstream != nil {
		oid, err := lib.NewOid(currentbranch.Upstream.Hash)
		if err != nil {
			return err
		}
		if err := walk.Push(oid); err != nil {
			return err
		}
	} else {
		if err := walk.Push(head.Target()); err != nil {
			return err
		}
	}

	defer walk.Free()

	err = walk.Iterate(func(commit *lib.Commit) bool {

		hash := commit.AsObject().Id().String()
		author := &Contributor{
			Name:  commit.Author().Name,
			Email: commit.Author().Email,
			When:  commit.Author().When,
		}
		sum := commit.Summary()
		msg := commit.Message()

		c := &Commit{
			commit:  commit,
			Hash:    hash,
			Author:  author,
			Message: msg,
			Summary: sum,
		}
		cs = append(cs, c)
		return true
	})
	r.Commits = cs
	return nil
}

func (c *Commit) String() string {
	return c.Hash
}

func (c *Commit) Date() string {
	return c.Author.When.String()
}

func (c *Commit) Since() string {
	return timeago.FromTime(c.Author.When)
}

func (c *Contributor) String() string {
	return c.Name + " " + "<" + c.Email + ">"
}

func (r *Repository) Diff(c *Commit) (*Diff, error) {
	// if c.commit.ParentCount() > 1 {
	// 	return nil, errors.New("commit has multiple parents")
	// }

	cTree, err := c.commit.Tree()
	if err != nil {
		return nil, err
	}
	defer cTree.Free()

	var pTree *lib.Tree
	if c.commit.ParentCount() > 0 {
		if pTree, err = c.commit.Parent(0).Tree(); err != nil {
			return nil, err
		}
		defer pTree.Free()
	}

	opt, err := lib.DefaultDiffOptions()
	if err != nil {
		return nil, err
	}

	diff, err := r.repo.DiffTreeToTree(pTree, cTree, &opt)
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
			Status: int(dd.Status),
			NewFile: &DiffFile{
				Path: dd.NewFile.Path,
				Hash: dd.NewFile.Oid.String(),
			},
			OldFile: &DiffFile{
				Path: dd.OldFile.Path,
				Hash: dd.OldFile.Oid.String(),
			},
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

func (r *Repository) DiffFromHash(hash string) (*Diff, error) {
	objectid, err := lib.NewOid(hash)
	if err != nil {
		return nil, err
	}
	c, err := r.repo.LookupCommit(objectid)
	if err != nil {
		return nil, err
	}
	return r.Diff(&Commit{commit: c})
}
