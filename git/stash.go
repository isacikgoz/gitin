package git

import (
	"fmt"

	lib "gopkg.in/libgit2/git2go.v27"
)

// StashedItem is a change that stashed into the repository
type StashedItem struct {
	essence *lib.Oid
	Index   int
	Hash    string
	Message string
}

// StashFlag is the flag that affect the stash save operation.
type StashFlag int

// See https://godoc.org/github.com/libgit2/git2go#StashFlag
const (
	StashDefault StashFlag = iota
	StashKeepIndex
	StashIncludeUntracked
	StashIncludeIgnored
)

// Stashes returns the stashed items of the repository
func (r *Repository) Stashes() ([]*StashedItem, error) {
	sc := r.essence.Stashes
	stashes := make([]*StashedItem, 0)
	sc.Foreach(func(index int, message string, id *lib.Oid) error {
		s := &StashedItem{
			Index:   index,
			Hash:    id.String(),
			Message: message,
			essence: id,
		}
		stashes = append(stashes, s)
		return nil
	})
	return stashes, nil
}

// AddToStash saves the modifications to stash
func (r *Repository) AddToStash(s *Signature, message string, flags StashFlag) (*StashedItem, error) {
	sc := r.essence.Stashes
	oid, err := sc.Save(s.toNewLibSignature(), message, lib.StashFlag(flags))
	if err != nil {
		return nil, fmt.Errorf("could not stash changes: %v", err)
	}
	stashes, err := r.Stashes()
	if err != nil {
		return nil, fmt.Errorf("could get stash: %v", err)
	}
	for _, s := range stashes {
		if oid.Equal(s.essence) {
			return s, nil
		}
	}
	return nil, fmt.Errorf("changes are not stashed")
}
