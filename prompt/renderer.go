package prompt

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/fatih/color"
	git "github.com/isacikgoz/libgit2-api"
)

var (
	// define colors
	green     = color.New(color.FgGreen)
	yellow    = color.New(color.FgYellow)
	red       = color.New(color.FgRed)
	cyan      = color.New(color.FgCyan)
	faint     = color.New(color.Faint)
	underline = color.New(color.Underline)
)

const (
	// date format could be defined by user
	dateFormat = "2006-01-02 15:04"
)

// PrintOptions tells the renderer to add author or date options
type PrintOptions struct {
	Date   bool
	Author bool
}

func renderLine(item Item, opts *PrintOptions) string {
	var line string
	switch item.(type) {
	case *git.StatusEntry:
		col := red
		entry := item.(*git.StatusEntry)
		if entry.Indexed() {
			col = green
		}
		ind := "[" + cyan.Sprint(entry.StatusEntryString()[:1]) + "]"
		line = fmt.Sprintf(" %s %s", ind, col.Sprint(entry))
	case *git.Commit:
		commit := item.(*git.Commit)
		hash := "[" + cyan.Sprint(commit.Hash[:7]) + "]"
		line = fmt.Sprintf(" %s %s", hash, item)
	case *git.DiffDelta:
		dd := item.(*git.DiffDelta)
		ind := "[" + cyan.Sprint(dd.DeltaStatusString()[:1]) + "]"
		line = fmt.Sprintf(" %s %s", ind, dd)
	default:
		line = fmt.Sprintf(" %s", item)
	}
	return line
}

func branchInfo(b *git.Branch) []string {
	if b == nil {
		return []string{faint.Sprint("Unable to load branch info")}
	}

	var str []string
	if len(b.Name) > 0 {
		str = []string{faint.Sprint("On branch ") + yellow.Sprint(b.Name)}
	}
	if b.Upstream == nil {
		return append(str, faint.Sprint("Your branch is not tracking a remote branch."))
	}
	pl := b.Behind
	ps := b.Ahead

	if ps == 0 && pl == 0 {
		str = append(str, faint.Sprint("Your branch is up to date with ")+cyan.Sprint(b.Upstream.Name)+faint.Sprint("."))
	} else {
		if ps > 0 && pl > 0 {
			str = append(str, faint.Sprint("Your branch and ")+cyan.Sprint(b.Upstream.Name)+faint.Sprint(" have diverged,"))
			str = append(str, faint.Sprint("and have ")+yellow.Sprint(strconv.Itoa(ps))+faint.Sprint(" and ")+yellow.Sprint(strconv.Itoa(pl))+faint.Sprint(" different commits each, respectively."))
			str = append(str, faint.Sprint("(\"pull\" to merge the remote branch into yours)"))
		} else if pl > 0 && ps == 0 {
			str = append(str, faint.Sprint("Your branch is behind ")+cyan.Sprint(b.Upstream.Name)+faint.Sprint(" by ")+yellow.Sprint(strconv.Itoa(pl))+faint.Sprint(" commit(s)."))
			str = append(str, faint.Sprint("(\"pull\" to update your local branch)"))
		} else if ps > 0 && pl == 0 {
			str = append(str, faint.Sprint("Your branch is ahead of ")+cyan.Sprint(b.Upstream.Name)+faint.Sprint(" by ")+yellow.Sprint(strconv.Itoa(ps))+faint.Sprint(" commit(s)."))
			str = append(str, faint.Sprint("(\"push\" to publish your local commits)"))
		}
	}
	return str
}

func branchClean(b *git.Branch) []string {
	var str []string
	str = append(str, branchInfo(b)...)
	str = append(str, faint.Sprint("Nothing to commit, working tree clean"))
	return str
}

func generateHelp(pairs map[string]string) []string {
	var arr []string
	// sort keys alphabetically
	keys := make([]string, 0, len(pairs))
	for key := range pairs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		arr = append(arr, fmt.Sprintf("%s: %s", faint.Sprint(key), yellow.Sprint(pairs[key])))
	}
	arr = append(arr, "")
	arr = append(arr, faint.Sprint("press any key to return."))
	return arr
}
