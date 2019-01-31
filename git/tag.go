package git

import (
	lib "gopkg.in/libgit2/git2go.v27"
)

// Tag is used to label and mark a specific commit in the history.
// It is usually used to mark release points
type Tag struct {
	Hash    string
	Target  *lib.Oid
	Tagger  *Contributor
	Name    string
	Message string
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

		if ref.IsTag() {
			tag, err := r.repo.LookupTag(ref.Target())
			if err != nil {

			} else {
				t := &Tag{
					Hash:    tag.Id().String(),
					Target:  tag.Target().Id(),
					Name:    tag.Name(),
					Message: tag.Message(),
					Tagger: &Contributor{
						Name:  tag.Tagger().Name,
						Email: tag.Tagger().Email,
						When:  tag.Tagger().When,
					},
				}
				ts = append(ts, t)
			}
		}
	}
	r.Tags = ts
	return ts, nil
}

// findTag looks up for the hash is targeted bu a tag
// this is a performance killer implementation. FIXME
func (r *Repository) findTag(hash string) *Tag {
	for _, t := range r.Tags {
		if t.Target.String()[:7] == hash[:7] {
			return t
		}
	}
	return nil
}
