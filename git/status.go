package git

import (
	"os/exec"
	"strings"

	git "gopkg.in/libgit2/git2go.v27"
	lib "gopkg.in/libgit2/git2go.v27"
)

// State is the current state of the repository
type State int

const (
	StateUnknown State = iota
	StateNone
	StateMerge
	StateRevert
	StateCherrypick
	StateBisect
	StateRebase
	StateRebaseInteractive
	StateRebaseMerge
	StateApplyMailbox
	StateApplyMailboxOrRebase
)

// IndexType describes the different stages a status entry can be in
type IndexType int

// The different status stages
const (
	IndexTypeStaged IndexType = iota
	IndexTypeUnstaged
	IndexTypeUntracked
	IndexTypeConflicted
)

// StatusEntryType describes the type of change a status entry has undergone
type StatusEntryType int

// The set of supported StatusEntryTypes
const (
	StatusEntryTypeUnmodified StatusEntryType = iota
	StatusEntryTypeAdded
	StatusEntryTypeDeleted
	StatusEntryTypeModified
	StatusEntryTypeRenamed
	StatusEntryTypeCopied
	StatusEntryTypeIgnored
	StatusEntryTypeUntracked
	StatusEntryTypeTypeChange
	StatusEntryTypeUnreadable
	StatusEntryTypeConflicted
)

// StatusEntry contains data for a single status entry
type StatusEntry struct {
	index           IndexType
	statusEntryType StatusEntryType
	diffDelta       *DiffDelta
}

// Status contains all git status data
type Status struct {
	State   State
	Entries []*StatusEntry
}

func (r *Repository) loadStatus() error {
	statusOptions := &lib.StatusOptions{
		Show:  lib.StatusShowIndexAndWorkdir,
		Flags: lib.StatusOptIncludeUntracked,
	}
	statusList, err := r.repo.StatusList(statusOptions)
	if err != nil {
		return err
	}
	defer statusList.Free()

	count, err := statusList.EntryCount()
	if err != nil {
		return err
	}
	entries := make([]*StatusEntry, 0)
	for i := 0; i < count; i++ {
		statusEntry, err := statusList.ByIndex(i)
		if err != nil {
			return err
		}
		if statusEntry.Status <= 0 {
			continue
		}
		index := getIndex(statusEntry.Status)
		var dd lib.DiffDelta
		if index == IndexTypeStaged {
			dd = statusEntry.HeadToIndex
		} else {
			dd = statusEntry.IndexToWorkdir
		}
		d := &DiffDelta{
			Status: int(dd.Status),
			NewFile: &DiffFile{
				Path: dd.NewFile.Path,
			},
			OldFile: &DiffFile{
				Path: dd.OldFile.Path,
			},
		}
		e := &StatusEntry{
			index:           index,
			statusEntryType: StatusEntryType(dd.Status),
			diffDelta:       d,
		}
		entries = append(entries, e)
	}
	s := &Status{
		State:   State(r.repo.State()),
		Entries: entries,
	}
	r.Status = s
	return nil
}

func getIndex(s lib.Status) IndexType {
	if s == git.StatusWtModified || s == git.StatusWtDeleted || s == git.StatusWtTypeChange || s == git.StatusWtRenamed {
		return IndexTypeUnstaged
	} else if s == git.StatusWtNew {
		return IndexTypeUntracked
	} else if s == git.StatusConflicted {
		return IndexTypeConflicted
	}
	return IndexTypeStaged
}

func (e *StatusEntry) String() string {
	return e.diffDelta.OldFile.Path
}

func (e *StatusEntry) Patch() string {
	cmd := exec.Command("git", "diff", (e.diffDelta.OldFile.Path))
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.Join(colorizeDiff(string(out)), "\n")
}

func (e *StatusEntry) StatusEntryString() string {
	switch e.statusEntryType {
	case StatusEntryTypeUnmodified:
		return ""
	case StatusEntryTypeAdded:
		return "Added"
	case StatusEntryTypeDeleted:
		return "Deleted"
	case StatusEntryTypeModified:
		return "Modified"
	case StatusEntryTypeRenamed:
		return "Renamed"
	case StatusEntryTypeCopied:
		return "Copied"
	case StatusEntryTypeIgnored:
		return "Ignored"
	case StatusEntryTypeUntracked:
		return "Untracked"
	case StatusEntryTypeTypeChange:
		return "Type change"
	case StatusEntryTypeUnreadable:
		return "Unreadable"
	case StatusEntryTypeConflicted:
		return "Conflicted"
	default:
		return "Unknown"
	}
}

func (e *StatusEntry) Indexed() bool {
	if e.index == IndexTypeStaged {
		return true
	}
	return false
}

func (r *Repository) AddEntry(e *StatusEntry) error {
	cmd := exec.Command("git", "add", "--", (e.diffDelta.OldFile.Path))
	if err := cmd.Run(); err != nil {
		return err
	}
	return r.loadStatus()
}

func (r *Repository) ResetEntry(e *StatusEntry) error {
	cmd := exec.Command("git", "reset", "HEAD", "--", (e.diffDelta.OldFile.Path))
	if err := cmd.Run(); err != nil {
		return err
	}
	return r.loadStatus()
}
