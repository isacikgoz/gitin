package git

import (
	"strconv"
	"strings"
	"time"
)

// Branch is a local or remote-tracking branch
type Branch struct {
	refType RefType

	Name     string
	FullName string
	Hash     string
	// When is the author date of the commit the branch points to
	When     time.Time
	isRemote bool
	Head     bool
	// Detached is set for the HEAD of a repository that is not on a branch
	Detached bool
	Ahead    int
	Behind   int
	Upstream *Branch
}

const branchFormat = "--format=%(refname)%00%(refname:lstrip=2)%00%(objectname)%00%(HEAD)%00%(upstream:lstrip=2)%00%(upstream)%00%(upstream:track,nobracket)%00%(authordate:unix)"

const branchFields = 8

// Branches loads the local and remote-tracking branches
func (r *Repository) Branches() ([]*Branch, error) {
	out, err := r.Output("for-each-ref", branchFormat, "refs/heads", "refs/remotes")
	if err != nil {
		return nil, err
	}
	var branches []*Branch
	for _, line := range lines(out) {
		f := strings.Split(line, "\x00")
		if len(f) != branchFields {
			continue
		}
		b := &Branch{
			refType:  RefTypeBranch,
			FullName: f[0],
			Name:     f[1],
			Hash:     f[2],
			Head:     f[3] == "*",
			isRemote: strings.HasPrefix(f[0], "refs/remotes/"),
		}
		if b.Head {
			b.refType = RefTypeHEAD
		}
		if ts, err := strconv.ParseInt(f[7], 10, 64); err == nil {
			b.When = time.Unix(ts, 0)
		}
		// an upstream that no longer exists is reported as "gone"
		if f[4] != "" && f[6] != "gone" {
			b.Upstream = &Branch{Name: f[4], FullName: f[5], isRemote: true, refType: RefTypeBranch}
			b.Ahead, b.Behind = parseTrack(f[6])
		}
		branches = append(branches, b)
	}
	return branches, nil
}

// parseTrack parses "ahead 1, behind 2" as printed by %(upstream:track,nobracket)
func parseTrack(track string) (ahead, behind int) {
	for _, part := range strings.Split(track, ", ") {
		if n, ok := strings.CutPrefix(part, "ahead "); ok {
			ahead, _ = strconv.Atoi(n)
		} else if n, ok := strings.CutPrefix(part, "behind "); ok {
			behind, _ = strconv.Atoi(n)
		}
	}
	return ahead, behind
}

// lines splits newline terminated output into lines
func lines(out []byte) []string {
	s := strings.TrimSuffix(string(out), "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// Type is the reference type of this ref
func (b *Branch) Type() RefType {
	return b.refType
}

func (b *Branch) String() string {
	return b.Name
}

// IsRemote returns false if it is a local branch
func (b *Branch) IsRemote() bool {
	return b.isRemote
}
