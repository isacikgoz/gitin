package prompt

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/justincampbell/timeago"

	"github.com/fatih/color"
	git "github.com/isacikgoz/libgit2-api"
)

var (
	// define colors
	green     = color.New(color.FgGreen)
	yellow    = color.New(color.FgYellow)
	blue      = color.New(color.FgBlue)
	red       = color.New(color.FgRed)
	cyan      = color.New(color.FgCyan)
	faint     = color.New(color.Faint)
	hiWhite   = color.New(color.FgHiWhite)
	bold      = color.New(color.Bold)
	whitebg   = color.New(color.BgWhite)
	blackbg   = color.New(color.BgBlack)
	underline = color.New(color.Underline)
	black     = color.New(color.FgBlack)
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
	if b.Upstream == nil {
		return []string{faint.Sprint("Your branch is not tracking a remote branch.")}
	}
	var str []string
	pl := b.Behind
	ps := b.Ahead

	if ps == 0 && pl == 0 {
		str = []string{faint.Sprint("Your branch is up to date with ") + cyan.Sprint(b.Upstream.Name) + faint.Sprint(".")}
	} else {
		if ps > 0 && pl > 0 {
			str = []string{faint.Sprint("Your branch and ") + cyan.Sprint(b.Upstream.Name) + faint.Sprint(" have diverged,")}
			str = append(str, faint.Sprint("and have ")+yellow.Sprint(strconv.Itoa(ps))+faint.Sprint(" and ")+yellow.Sprint(strconv.Itoa(pl))+faint.Sprint(" different commits each, respectively."))
			str = append(str, faint.Sprint("(\"pull\" to merge the remote branch into yours)"))
		} else if pl > 0 && ps == 0 {
			str = []string{faint.Sprint("Your branch is behind ") + cyan.Sprint(b.Upstream.Name) + faint.Sprint(" by ") + yellow.Sprint(strconv.Itoa(pl)) + faint.Sprint(" commit(s).")}
			str = append(str, faint.Sprint("(\"pull\" to update your local branch)"))
		} else if ps > 0 && pl == 0 {
			str = []string{faint.Sprint("Your branch is ahead of ") + cyan.Sprint(b.Upstream.Name) + faint.Sprint(" by ") + yellow.Sprint(strconv.Itoa(ps)) + faint.Sprint(" commit(s).")}
			str = append(str, faint.Sprint("(\"push\" to publish your local commits)"))
		}
	}
	return str
}

func branchClean(b *git.Branch) []string {
	str := []string{faint.Sprint("On branch ") + yellow.Sprint(b.Name)}
	str = append(str, branchInfo(b)...)
	str = append(str, faint.Sprint("Nothing to commit, working tree clean"))
	return str
}

func logInfo(item Item) []string {
	str := make([]string, 0)
	if item == nil {
		return str
	}
	switch item.(type) {
	case *git.Commit:
		commit := item.(*git.Commit)
		str = append(str, faint.Sprint("Author")+" "+commit.Author.Name+" <"+commit.Author.Email+">")
		str = append(str, faint.Sprint("When")+"   "+timeago.FromTime(commit.Author.When))
		return str
	case *git.DiffDelta:
		dd := item.(*git.DiffDelta)
		var adds, dels int
		for _, line := range strings.Split(dd.Patch, "\n") {
			if len(line) > 0 {
				switch rn := line[0]; rn {
				case '+':
					adds++
				case '-':
					dels++
				}
			}
		}
		var infoLine string
		if adds > 1 {
			infoLine = fmt.Sprintf("%s %s", green.Sprintf("%d", adds-1), faint.Sprint("additions"))
		}
		if dels > 1 {
			if len(infoLine) > 1 {
				infoLine = infoLine + " "
			}
			infoLine = infoLine + fmt.Sprintf("%s %s", red.Sprintf("%d", dels-1), faint.Sprint("deletions"))
		}
		if len(infoLine) > 1 {
			infoLine = infoLine + faint.Sprint(".")
		}
		str = append(str, infoLine)
	}
	return str
}
