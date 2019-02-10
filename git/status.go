package git

import (
	"os/exec"
	"strings"

	log "github.com/sirupsen/logrus"
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
	StatusEntryTypeNew StatusEntryType = iota
	StatusEntryTypeModified
	StatusEntryTypeDeleted
	StatusEntryTypeRenamed
	StatusEntryTypeUntracked
	StatusEntryTypeTypeChange
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
	State    State
	Entities map[IndexType][]*StatusEntry
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
	entities := make(map[IndexType][]*StatusEntry)
	s := &Status{
		State:    State(r.repo.State()),
		Entities: entities,
	}
	for i := 0; i < count; i++ {
		statusEntry, err := statusList.ByIndex(i)
		if err != nil {
			return err
		}
		if statusEntry.Status <= 0 {
			continue
		}
		s.addToStatus(statusEntry)
	}
	r.Status = s
	return nil
}

var indexTypeMap = map[lib.Status]IndexType{
	lib.StatusIndexNew | lib.StatusIndexModified | lib.StatusIndexDeleted | lib.StatusIndexRenamed | lib.StatusIndexTypeChange: IndexTypeStaged,
	lib.StatusWtModified | lib.StatusWtDeleted | lib.StatusWtTypeChange | lib.StatusWtRenamed:                                  IndexTypeUnstaged,
	lib.StatusWtNew:      IndexTypeUntracked,
	lib.StatusConflicted: IndexTypeConflicted,
}

var statusEntryTypeMap = map[lib.Status]StatusEntryType{
	lib.StatusIndexNew:        StatusEntryTypeNew,
	lib.StatusIndexModified:   StatusEntryTypeModified,
	lib.StatusWtModified:      StatusEntryTypeModified,
	lib.StatusIndexDeleted:    StatusEntryTypeDeleted,
	lib.StatusWtDeleted:       StatusEntryTypeDeleted,
	lib.StatusIndexRenamed:    StatusEntryTypeRenamed,
	lib.StatusWtRenamed:       StatusEntryTypeRenamed,
	lib.StatusIndexTypeChange: StatusEntryTypeTypeChange,
	lib.StatusWtTypeChange:    StatusEntryTypeTypeChange,
	lib.StatusWtNew:           StatusEntryTypeUntracked,
	lib.StatusConflicted:      StatusEntryTypeConflicted,
}

func (s *Status) addToStatus(raw git.StatusEntry) {
	for rawStatus, indexType := range indexTypeMap {
		processedRawStatus := raw.Status & rawStatus

		if processedRawStatus > 0 {
			if _, ok := s.Entities[indexType]; !ok {
				statusEntries := make([]*StatusEntry, 0)
				s.Entities[indexType] = statusEntries
			}
			e, err := newEntry(raw, indexType, processedRawStatus)
			if err != nil {
				continue
			}
			s.Entities[indexType] = append(s.Entities[indexType], e)
		}
	}
}

func newEntry(raw git.StatusEntry, index IndexType, set lib.Status) (*StatusEntry, error) {
	var dd lib.DiffDelta
	if index == IndexTypeStaged {
		dd = raw.HeadToIndex
	} else {
		dd = raw.IndexToWorkdir
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
		statusEntryType: statusEntryTypeMap[set],
		diffDelta:       d,
	}
	return e, nil
}

func (e *StatusEntry) String() string {
	if len(e.diffDelta.OldFile.Path) <= 0 {
		return e.diffDelta.NewFile.Path
	}
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
	if e.index == IndexTypeStaged {
		args = []string{"diff", "--cached", e.diffDelta.OldFile.Path}
	} else if e.statusEntryType == StatusEntryTypeUntracked {
		args = []string{"diff", "--no-index", "/dev/null", e.diffDelta.NewFile.Path}
	} else {
		args = []string{"diff", "--", e.diffDelta.OldFile.Path}
	}
	return args
}

// StatusEntryString returns entry status in pretty format
func (e *StatusEntry) StatusEntryString() string {
	switch e.statusEntryType {
	case StatusEntryTypeNew:
		return "Added"
	case StatusEntryTypeDeleted:
		return "Deleted"
	case StatusEntryTypeModified:
		return "Modified"
	case StatusEntryTypeRenamed:
		return "Renamed"
	case StatusEntryTypeUntracked:
		return "Untracked"
	case StatusEntryTypeTypeChange:
		return "Type change"
	case StatusEntryTypeConflicted:
		return "Conflicted"
	default:
		return "Unknown"
	}
}

// Indexed true if entry added to index
func (e *StatusEntry) Indexed() bool {
	return e.index == IndexTypeStaged
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
