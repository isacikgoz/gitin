package git

import (
	"bufio"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/justincampbell/timeago"
	log "github.com/sirupsen/logrus"
	lib "gopkg.in/libgit2/git2go.v27"
)

// Commit is the wrapper of actual lib.Commit object
type Commit struct {
	commit  *lib.Commit
	Hash    string
	Author  *Contributor
	Message string
	Summary string
	Type    CommitType
	Tag     *Tag
	Heads   []*Branch
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

// CommitLoadOptions holds limitation while loading commits from the store
type CommitLoadOptions struct {
	Author    string
	Before    string
	Committer string
	MaxCount  int
	Tags      bool
	Since     string
}

func (r *Repository) loadCommits(from, to *lib.Oid, opts *CommitLoadOptions) ([]*Commit, error) {
	cs := make([]*Commit, 0)
	if to != nil && from.Equal(to) {
		return cs, nil
	}
	walk, err := r.repo.Walk()
	if err != nil {
		return cs, err
	}
	if err := walk.Push(from); err != nil {
		return cs, err
	}

	defer walk.Free()
	counter := 0
	limit := datefilter(opts) || signaturefilter(opts)
	err = walk.Iterate(func(commit *lib.Commit) bool {
		oid := commit.AsObject().Id()
		if to != nil && to.Equal(oid) {
			return false
		}

		hash := oid.String()
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

		if tag := r.findTag(c.Hash); tag != nil {
			c.Tag = tag
		}

		if limit {
			if ok, _ := limitCommit(commit, opts); ok {
				counter++
				cs = append(cs, c)
			}
		} else {
			counter++
			cs = append(cs, c)
		}

		if opts.MaxCount != 0 && counter >= opts.MaxCount {
			return false
		}
		return true
	})
	r.Commits = cs
	return cs, nil
}

// failOverShallow is a backdoor to load commits. Since gitlib.v27 cannot load
// shallow repositories this is a failover mechanism that uses actual git commands
// TODO: Fix magic numbers
func (r *Repository) failOverShallow(opts *CommitLoadOptions) ([]*Commit, error) {
	file, err := os.Open(".git/shallow")
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(file)
	var shallow string
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 40 {
			shallow = line
			break
		}
	}
	defer file.Close()
	cmd := exec.Command("git", "rev-list", "--all")
	cmd.Dir = r.AbsPath
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	commits := make([]*Commit, 0)
	re := regexp.MustCompile(`\r?\n`)
	counter := 0
	limit := datefilter(opts) || signaturefilter(opts)
	for _, line := range re.Split(string(out), -1) {
		if line[:7] == shallow[:7] {
			break
		}
		objectid, err := lib.NewOid(line)
		if err != nil {
			continue
		}
		commit, err := r.repo.LookupCommit(objectid)
		if err != nil {
			continue
		}
		if limit {
			if ok, _ := limitCommit(commit, opts); !ok {
				continue
			}
		}

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
		commits = append(commits, c)
		counter++
		if opts.MaxCount != 0 && counter >= opts.MaxCount {
			return commits, nil
		}
	}
	return commits, nil
}
func (c *Commit) String() string {
	return c.Hash
}

// Date returns the commits's creation date as string
func (c *Commit) Date() string {
	return c.Author.When.String()
}

// Since returns xx ago string
func (c *Commit) Since() string {
	return timeago.FromTime(c.Author.When)
}

func (c *Contributor) String() string {
	return c.Name + " " + "<" + c.Email + ">"
}

// Diff is the equivelant of "git diff <commit>", but it is restricted to commits
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

// DiffFromHash is a wrapper for Actual diff which takes a hash string for input
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

// revlist is the wrapped of "git rev-list oid1..oid2" command
func (r *Repository) revlist(from, to *lib.Oid) ([]*Commit, error) {
	commits := make([]*Commit, 0)

	cmd := exec.Command("git", "rev-list", from.String()+".."+to.String())
	cmd.Dir = r.AbsPath
	out, err := cmd.Output()
	if err != nil {
		return commits, err
	}
	output := string(out)
	output = strings.TrimSpace(output)
	re := regexp.MustCompile(`\r?\n`)
	for _, line := range re.Split(output, -1) {
		objectid, err := lib.NewOid(line)
		if err != nil {
			continue
		}
		commit, err := r.repo.LookupCommit(objectid)
		if err != nil {
			continue
		}
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
		commits = append(commits, c)
	}
	return commits, nil
}

// TODO: performance improvement required, parse dates before limit
func limitCommit(commit *lib.Commit, opts *CommitLoadOptions) (bool, error) {
	if len(opts.Author) > 0 {
		sign := commit.Author().Name + " <" + commit.Author().Email + ">"
		if strings.Contains(sign, opts.Author) {
			return true, nil
		}
		return false, nil
	}
	if len(opts.Committer) > 0 {
		sign := commit.Committer().Name + " <" + commit.Committer().Email + ">"
		if strings.Contains(sign, opts.Committer) {
			return true, nil
		}
		return false, nil
	}
	if len(opts.Before) > 0 {
		cdate := commit.Author().When
		udate, err := time.Parse(time.RFC3339, opts.Before)
		if err != nil {
			return false, err
		}
		if cdate.Before(udate) {
			return true, nil
		}
		return false, nil
	}
	if len(opts.Since) > 0 {
		cdate := commit.Author().When
		udate, err := time.Parse(time.RFC3339, opts.Since)
		if err != nil {
			return false, err
		}
		if cdate.After(udate) {
			return true, nil
		}
		return false, nil
	}
	return true, nil
}

func datefilter(opts *CommitLoadOptions) bool {
	if len(opts.Since) > 0 || len(opts.Before) > 0 {
		return true
	}
	return false
}

func signaturefilter(opts *CommitLoadOptions) bool {
	if len(opts.Author) > 0 || len(opts.Committer) > 0 {
		return true
	}
	return false
}

// Decoration returns the string if the commit has tag or reference
func (c *Commit) Decoration() string {
	var decor string
	if c.Tag != nil {
		decor = "(tag: " + c.Tag.Shorthand + ")"
	}
	return decor
}

// LastCommitStat prints the stat of the last commit
func (r *Repository) LastCommitStat() string {
	head, err := r.repo.Head()
	if err != nil {
		return "error reading last commit"
	}
	commit, err := r.repo.LookupCommit(head.Target())
	if err != nil {
		return "error reading last commit"
	}
	hash := commit.AsObject().Id().String()
	cmd := exec.Command("git", "show", "--stat", hash)
	cmd.Dir = r.AbsPath
	out, err := cmd.Output()
	if err != nil {
		return err.Error()
	}
	return string(out)
}

// LastCommitArgs returns the args for show stat
func (r *Repository) LastCommitArgs() []string {
	head, err := r.repo.Head()
	if err != nil {
		log.Error(err)
		return nil
	}
	hash := string(head.Target().String())
	args := []string{"show", "--stat", hash}
	return args
}
