package git

// Tag is used to label and mark a specific commit in the history.
// It is usually used to mark release points
type Tag struct {
	Hash string
	// Target    *lib.Oid
	// Tagger    *Contributor
	Shorthand string
	Name      string
	refType   RefType
	target    string
	// Message   string
}

// loadTags loads tags from the refs
func (r *Repository) loadTags() ([]*Tag, error) {
	ts := make([]*Tag, 0)

	iter, err := r.repo.NewReferenceIterator()
	if err != nil {
		return ts, err
	}
	defer iter.Free()

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
			if _, ok := r.RefMap[t.Hash]; !ok {
				r.RefMap[t.Hash] = make([]Ref, 0)
			}
			refs := r.RefMap[t.Hash]
			refs = append(refs, t)
			r.RefMap[t.Hash] = refs
			ts = append(ts, t)

		}
	}
	r.Tags = ts
	return ts, nil
}

// findTag looks up for the hash is targeted bu a tag
// this is a performance killer implementation. FIXME
func (r *Repository) findTag(hash string) *Tag {
	for _, t := range r.Tags {
		if t.Hash[:7] == hash[:7] {
			return t
		}
	}
	return nil
}

func (t *Tag) Type() RefType {
	return t.refType
}

func (t *Tag) Target() string {
	return t.target
}

func (t *Tag) Display() string {
	return t.Shorthand
}

func (t *Tag) Oid() string {
	return t.Hash
}

func (t *Tag) ShortType() rune {
	return 't'
}
