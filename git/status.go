package git

import (
	"fmt"
	"strconv"
	"strings"
)

// DeltaStatus ondicates a files status in a diff
type DeltaStatus int

// Delta status of a file e.g. on a commit
const (
	DeltaUnmodified DeltaStatus = iota
	DeltaAdded
	DeltaDeleted
	DeltaModified
	DeltaRenamed
	DeltaCopied
	DeltaIgnored
	DeltaUntracked
	DeltaTypeChange
	DeltaUnreadable
	DeltaConflicted
)

// deltaStatusFromCode maps a status letter of "git diff --raw" to a DeltaStatus
func deltaStatusFromCode(code byte) DeltaStatus {
	switch code {
	case 'A':
		return DeltaAdded
	case 'D':
		return DeltaDeleted
	case 'M':
		return DeltaModified
	case 'R':
		return DeltaRenamed
	case 'C':
		return DeltaCopied
	case 'T':
		return DeltaTypeChange
	case 'U':
		return DeltaConflicted
	default:
		return DeltaUnreadable
	}
}

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

// statusEntryTypeFromCode maps a status letter of "git status --porcelain" to a StatusEntryType
func statusEntryTypeFromCode(code byte) StatusEntryType {
	switch code {
	case 'A':
		return StatusEntryTypeNew
	case 'D':
		return StatusEntryTypeDeleted
	case 'R':
		return StatusEntryTypeRenamed
	case 'T':
		return StatusEntryTypeTypeChange
	default:
		return StatusEntryTypeModified
	}
}

// StatusEntry contains data for a single status entry
type StatusEntry struct {
	index     IndexType
	EntryType StatusEntryType
	path      string
}

// Status contains all git status data
type Status struct {
	// Branch is the checked out branch compared to its upstream
	Branch   *Branch
	Entities []*StatusEntry
}

// Diff is the list of files changed by a commit
type Diff struct {
	deltas []*DiffDelta
}

// Deltas returns the actual changes with file info
func (d *Diff) Deltas() []*DiffDelta {
	return d.deltas
}

// DiffDelta holds delta status, file changes and the number of changed lines
type DiffDelta struct {
	Status    DeltaStatus
	OldFile   *DiffFile
	NewFile   *DiffFile
	Additions int
	Deletions int
	Binary    bool
	Commit    *Commit
}

// DiffFile the file that has been changed
type DiffFile struct {
	Path string
	Hash string
}

func (d *DiffDelta) String() string {
	return d.OldFile.Path
}

// LoadStatus simply emulates a "git status" and returns the result
func (r *Repository) LoadStatus() (*Status, error) {
	// like "git status", this refreshes the index so later calls are fast
	out, err := r.Output("status", "--porcelain=v2", "-z", "--branch", "--untracked-files=normal", "--no-renames")
	if err != nil {
		return nil, err
	}
	return parseStatus(out)
}

func parseStatus(out []byte) (*Status, error) {
	s := &Status{Branch: &Branch{refType: RefTypeHEAD, Head: true}}
	records := strings.Split(string(out), "\x00")
	tracking := false
	for i := 0; i < len(records); i++ {
		record := records[i]
		if record == "" {
			continue
		}
		switch record[0] {
		case '#':
			s.Branch.parseHeader(record)
			tracking = tracking || strings.HasPrefix(record, "# branch.ab ")
		case '1', '2', 'u':
			fields := map[byte]int{'1': 9, '2': 10, 'u': 11}[record[0]]
			f := strings.SplitN(record, " ", fields)
			if len(f) != fields || len(f[1]) != 2 {
				return nil, fmt.Errorf("unexpected status record %q", record)
			}
			xy, path := f[1], f[fields-1]
			if record[0] == '2' {
				i++ // the original path of a rename follows as its own record
			}
			if record[0] == 'u' {
				s.add(IndexTypeConflicted, StatusEntryTypeConflicted, path)
				continue
			}
			if xy[0] != '.' {
				s.add(IndexTypeStaged, statusEntryTypeFromCode(xy[0]), path)
			}
			if xy[1] != '.' {
				s.add(IndexTypeUnstaged, statusEntryTypeFromCode(xy[1]), path)
			}
		case '?':
			s.add(IndexTypeUntracked, StatusEntryTypeUntracked, record[2:])
		}
	}
	// an upstream that no longer exists has no ahead/behind counts
	if !tracking {
		s.Branch.Upstream = nil
	}
	return s, nil
}

func (s *Status) add(index IndexType, entryType StatusEntryType, path string) {
	s.Entities = append(s.Entities, &StatusEntry{index: index, EntryType: entryType, path: path})
}

// parseHeader reads the "# branch.*" headers of "git status --porcelain=v2 --branch"
func (b *Branch) parseHeader(header string) {
	key, value, _ := strings.Cut(strings.TrimPrefix(header, "# "), " ")
	switch key {
	case "branch.oid":
		if value != "(initial)" {
			b.Hash = value
		}
	case "branch.head":
		if value == "(detached)" {
			b.Detached = true
		} else {
			b.Name = value
			b.FullName = "refs/heads/" + value
		}
	case "branch.upstream":
		b.Upstream = &Branch{Name: value, isRemote: true, refType: RefTypeBranch}
	case "branch.ab":
		for _, n := range strings.Fields(value) {
			v, _ := strconv.Atoi(n[1:])
			if n[0] == '+' {
				b.Ahead = v
			} else {
				b.Behind = v
			}
		}
	}
}

// Indexed true if entry added to index
func (e *StatusEntry) String() string {
	return e.path
}

// Indexed true if entry added to index
func (e *StatusEntry) Indexed() bool {
	return e.index == IndexTypeStaged
}

// StatusEntryString returns entry status in pretty format
func (e *StatusEntry) StatusEntryString() string {
	switch e.EntryType {
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

// DeltaStatusString retruns delta status as string
func (d *DiffDelta) DeltaStatusString() string {
	switch d.Status {
	case DeltaUnmodified:
		return "Unmodified"
	case DeltaAdded:
		return "Added"
	case DeltaDeleted:
		return "Deleted"
	case DeltaModified:
		return "Modified"
	case DeltaRenamed:
		return "Renamed"
	case DeltaCopied:
		return "Copied"
	case DeltaIgnored:
		return "Ignored"
	case DeltaUntracked:
		return "Untracked"
	case DeltaTypeChange:
		return "TypeChange"
	case DeltaUnreadable:
		return "Unreadable"
	case DeltaConflicted:
		return "Conflicted"
	}
	return " "
}
