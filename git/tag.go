package git

import "strings"

// Tag is used to label and mark a specific commit in the history.
type Tag struct {
	refType RefType

	// Hash is the commit the tag points to, annotated tags are peeled
	Hash      string
	Shorthand string
	Name      string
}

// Type is the reference type of this ref
func (t *Tag) Type() RefType {
	return t.refType
}

func (t *Tag) String() string {
	return t.Shorthand
}

const refFormat = "--format=%(refname)%00%(refname:lstrip=2)%00%(objectname)%00%(*objectname)%00%(HEAD)"

// Refs maps commit hashes to the branches and tags pointing at them. It is
// lighter than Branches since it does not compare branches to upstreams.
func (r *Repository) Refs() (map[string][]Ref, error) {
	out, err := r.Output("for-each-ref", refFormat, "refs/heads", "refs/remotes", "refs/tags")
	if err != nil {
		return nil, err
	}
	refs := make(map[string][]Ref)
	for _, line := range lines(out) {
		f := strings.Split(line, "\x00")
		if len(f) != 5 {
			continue
		}
		name, shorthand, hash := f[0], f[1], f[2]
		if f[3] != "" {
			hash = f[3]
		}
		var ref Ref
		switch {
		case strings.HasPrefix(name, "refs/tags/"):
			ref = &Tag{refType: RefTypeTag, Name: name, Shorthand: shorthand, Hash: hash}
		case f[4] == "*":
			ref = &Branch{refType: RefTypeHEAD, Name: shorthand, FullName: name, Hash: hash, Head: true}
		default:
			isRemote := strings.HasPrefix(name, "refs/remotes/")
			ref = &Branch{refType: RefTypeBranch, Name: shorthand, FullName: name, Hash: hash, isRemote: isRemote}
		}
		refs[hash] = append(refs[hash], ref)
	}
	return refs, nil
}
