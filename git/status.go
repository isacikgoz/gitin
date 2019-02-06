package git

import (
	"os/exec"
	"strings"

	log "github.com/sirupsen/logrus"
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
	if s == lib.StatusWtModified || s == lib.StatusWtDeleted || s == lib.StatusWtTypeChange || s == lib.StatusWtRenamed {
		return IndexTypeUnstaged
	} else if s == lib.StatusWtNew {
		return IndexTypeUntracked
	} else if s == lib.StatusConflicted {
		return IndexTypeConflicted
	}
	return IndexTypeStaged
}

func (e *StatusEntry) String() string {
	return e.diffDelta.OldFile.Path
}

// Patch return the diff of the entry
func (r *Repository) Patch(e *StatusEntry) string {
	var cmd *exec.Cmd
	if e.statusEntryType == StatusEntryTypeUntracked {
		cmd = exec.Command("git", "diff", "--no-index", "/dev/null", e.diffDelta.NewFile.Path)
	} else {
		cmd = exec.Command("git", "diff", e.diffDelta.OldFile.Path)
	}
	cmd.Dir = r.AbsPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Warn(err.Error())
	}
	return strings.Join(colorizeDiff(string(out)), "\n")
}

// FileStatArgs returns git command args for getting diff
func (e *StatusEntry) FileStatArgs() []string {
	var args []string
	if e.statusEntryType == StatusEntryTypeUntracked {
		args = []string{"diff", "--no-index", "/dev/null", e.diffDelta.NewFile.Path}
	} else {
		args = []string{"diff", e.diffDelta.OldFile.Path}
	}
	return args
}

// StatusEntryString returns entry status in pretty format
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

// Indexed true if entry added to index
func (e *StatusEntry) Indexed() bool {
	if e.index == IndexTypeStaged {
		return true
	}
	return false
}

// AddEntry is the wrapper of "git add /path/to/file" command
func (r *Repository) AddEntry(e *StatusEntry) error {
	cmd := exec.Command("git", "add", "--", e.diffDelta.OldFile.Path)
	cmd.Dir = r.AbsPath
	if err := cmd.Run(); err != nil {
		return err
	}
	return r.loadStatus()
}

// ResetEntry is the wrapper of "git reset path/to/file" command
func (r *Repository) ResetEntry(e *StatusEntry) error {
	cmd := exec.Command("git", "reset", "HEAD", "--", e.diffDelta.OldFile.Path)
	cmd.Dir = r.AbsPath
	if err := cmd.Run(); err != nil {
		return err
	}
	return r.loadStatus()
}

// AddAll is the wrapper of "git add ." command
func (r *Repository) AddAll() error {
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = r.AbsPath
	if err := cmd.Run(); err != nil {
		return err
	}
	return r.loadStatus()
}

// ResetAll is the wrapper of "git reset" command
func (r *Repository) ResetAll() error {
	cmd := exec.Command("git", "reset", "--mixed")
	cmd.Dir = r.AbsPath
	if err := cmd.Run(); err != nil {
		return err
	}
	return r.loadStatus()
}

// NumberOfIndexedEntries returns the count of indexed files in the working dir
func (r *Repository) NumberOfIndexedEntries() int {
	count := 0
	for _, e := range r.Status.Entries {
		if e.Indexed() {
			count++
		}
	}
	return count
}
