package git

// Tag is used to label and mark a specific commit in the history.
type Tag struct {
	target  *Commit
	refType RefType

	Hash      string
	Shorthand string
	Name      string
}

// Tags loads tags from the refs
func (r *Repository) Tags() ([]*Tag, error) {

	iter, err := r.essence.NewReferenceIterator()
	if err != nil {
		return nil, err
	}
	defer iter.Free()
	buffer := make([]*Tag, 0)
	for {
		ref, err := iter.Next()
		if err != nil || ref == nil {
			break
		}

		if !ref.IsRemote() && ref.IsTag() {

			t := &Tag{
				Hash:      ref.Target().String(),
				refType:   RefTypeTag,
				Name:      ref.Name(),
				Shorthand: ref.Shorthand(),
			}
			// add to refmap
			if _, ok := r.RefMap[t.Hash]; !ok {
				r.RefMap[t.Hash] = make([]Ref, 0)
			}
			refs := r.RefMap[t.Hash]
			refs = append(refs, t)
			r.RefMap[t.Hash] = refs

			obj, err := r.essence.RevparseSingle(ref.Target().String())
			if err == nil && obj != nil {
				if commit, _ := obj.AsCommit(); commit != nil {
					t.target = unpackRawCommit(r, commit)
				}
			}
			buffer = append(buffer, t)
		}
	}
	return buffer, nil
}

// Type is the reference type of this ref
func (t *Tag) Type() RefType {
	return t.refType
}

// Target is the hash of targeted commit
func (t *Tag) Target() *Commit {
	return t.target
}

func (t *Tag) String() string {
	return t.Shorthand
}
