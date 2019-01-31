package git

import (
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	lib "gopkg.in/libgit2/git2go.v27"
)

// Branch is simply a lightweight movable pointer to one of repositories' commits
type Branch struct {
	Name       string
	FullName   string
	Hash       string
	Upstream   *Branch
	Ahead      []*Commit
	Behind     []*Commit
	Clean      bool
	isRemote   bool
	lastCommit *Commit
}

// loadBranches loads branches with the lib's branch iterator loads both remote and
// local branches
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
				var err1, err2 error
				aheads, err1 = r.revlist(us.Target(), branch.Reference.Target())
				behinds, err2 = r.revlist(branch.Reference.Target(), us.Target())
				if err1 != nil || err2 != nil {
					a, b, err := r.repo.AheadBehind(branch.Reference.Target(), us.Target())
					if err == nil {
						aheads = make([]*Commit, a)
						behinds = make([]*Commit, b)
					}
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
		if b.lastCommit == nil {
			objectid, err := lib.NewOid(b.Hash)
			if err != nil {
			} else {
				commit, err := r.repo.LookupCommit(objectid)
				if err != nil {
				} else {
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
					b.lastCommit = c
				}
			}
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

// Status genrates a string similar to "git status"
func (b *Branch) Status() string {
	if b.isRemote {
		return ""
	}
	if b.Upstream == nil || b.Ahead == nil || b.Behind == nil {
		return "This branch is not tracking a remote branch."
	}
	var str string
	pl := len(b.Behind)
	ps := len(b.Ahead)
	if ps == 0 && pl == 0 {
		str = "This branch is up to date with " + b.Upstream.Name + "."
	} else {
		if ps > 0 && pl > 0 {
			str = "This branch and " + b.Upstream.Name + " have diverged,"
			str = str + "\n" + "and have " + strconv.Itoa(ps) + " and " + strconv.Itoa(pl) + " different commits each, respectively."
			str = str + "\n" + "(\"pull\" to merge the remote branch into this branch)"
		} else if pl > 0 && ps == 0 {
			str = "This branch is behind " + b.Upstream.Name + " by " + strconv.Itoa(pl) + " commit(s)."
			str = str + "\n" + "(\"pull\" to update this local branch)"
		} else if ps > 0 && pl == 0 {
			str = "This branch is ahead of " + b.Upstream.Name + " by " + strconv.Itoa(ps) + " commit(s)."
			str = str + "\n" + "(\"push\" to publish this local commits)"
		}
	}
	return str
}

// LastCommitMessage returns the message of the targeted commit by this branch
func (b *Branch) LastCommitMessage() string {
	if b.lastCommit != nil {
		return b.lastCommit.Summary
	}
	return ""
}

// LastCommitDate returns the date of the targeted commit by this branch
func (b *Branch) LastCommitDate() string {
	if b.lastCommit != nil {
		return b.lastCommit.Date()
	}
	return ""
}

// LastCommitAuthor returns the author of the targeted commit by this branch
func (b *Branch) LastCommitAuthor() string {
	if b.lastCommit != nil {
		return b.lastCommit.Author.String()
	}
	return ""
}

// IsRemote is true if the ref is a remote ref
func (b *Branch) IsRemote() bool {
	return b.isRemote
}
