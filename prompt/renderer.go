package prompt

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/fatih/color"
	"github.com/isacikgoz/gitin/term"
	git "github.com/isacikgoz/libgit2-api"
)

const (
	// date format could be defined by user
	dateFormat = "2006-01-02 15:04"
)

func renderItem(item Item, matches []int, selected bool) []term.Cell {
	var line []term.Cell
	if selected {
		line = append(line, term.Cprint("> ", color.FgCyan)...)
	} else {
		line = append(line, term.Cprint("  ", color.FgWhite)...)
	}
	switch item.(type) {
	case *git.StatusEntry:
		attr := color.FgRed
		entry := item.(*git.StatusEntry)
		if entry.Indexed() {
			attr = color.FgGreen
		}
		line = append(line, stautsText(entry.StatusEntryString()[:1])...)
		line = append(line, highLightedText(matches, attr, entry.String())...)
	case *git.Commit:
		commit := item.(*git.Commit)
		line = append(line, stautsText(commit.Hash[:7])...)
		line = append(line, highLightedText(matches, color.FgWhite, commit.String())...)
	case *git.DiffDelta:
		dd := item.(*git.DiffDelta)
		line = append(line, stautsText(dd.DeltaStatusString()[:1])...)
		line = append(line, highLightedText(matches, color.FgWhite, dd.String())...)
	default:
		line = append(line, highLightedText(matches, color.FgWhite, item.String())...)
	}
	return line
}

func renderSearch(placeholder string, inputMode bool, input string) []term.Cell {
	var cells []term.Cell
	if inputMode {
		cells = term.Cprint("Search ", color.Faint)
		cells = append(cells, term.Cprint(placeholder+" ", color.Faint)...)
		cells = append(cells, term.Cprint(input, color.FgWhite)...)
		cells = append(cells, term.Cprint("█", color.Faint, color.BlinkRapid)...)
		return cells
	}
	cells = term.Cprint(placeholder, color.Faint)
	if len(input) > 0 {
		cells = append(cells, term.Cprint(" /"+input, color.FgWhite)...)
	}

	return cells
}

func stautsText(text string) []term.Cell {
	var cells []term.Cell
	if len(text) == 0 {
		return cells
	}
	cells = append(cells, term.Cell{Ch: '['})
	cells = append(cells, term.Cprint(text, color.FgCyan)...)
	cells = append(cells, term.Cell{Ch: ']'})
	cells = append(cells, term.Cell{Ch: ' '})
	return cells
}

func highLightedText(matches []int, c color.Attribute, str string) []term.Cell {
	if len(matches) == 0 {
		return term.Cprint(str, c)
	}
	highligted := make([]term.Cell, 0)
	for _, r := range str {
		highligted = append(highligted, term.Cell{
			Ch:   r,
			Attr: []color.Attribute{c},
		})
	}
	for _, m := range matches {
		if m > len(highligted)-1 {
			continue
		}
		highligted[m] = term.Cell{
			Ch:   highligted[m].Ch,
			Attr: append(highligted[m].Attr, color.Underline),
		}
	}
	return highligted
}

func branchInfo(b *git.Branch, yours bool) [][]term.Cell {
	sal := "This"
	if yours {
		sal = "Your"
	}
	var grid [][]term.Cell
	if b == nil {
		return append(grid, term.Cprint("Unable to load branch info", color.Faint))
	}
	if yours && len(b.Name) > 0 {
		bName := term.Cprint("On branch ", color.Faint)
		bName = append(bName, term.Cprint(b.Name, color.FgYellow)...)
		grid = append(grid, bName)
	}
	if b.Upstream == nil {
		return append(grid, term.Cprint(sal+" branch is not tracking a remote branch.", color.Faint))
	}
	pl := b.Behind
	ps := b.Ahead
	if ps == 0 && pl == 0 {
		cells := term.Cprint(sal+" branch is up to date with ", color.Faint)
		cells = append(cells, term.Cprint(b.Upstream.Name, color.FgCyan)...)
		cells = append(cells, term.Cprint(".", color.Faint)...)
		grid = append(grid, cells)
	} else {
		ucs := term.Cprint(b.Upstream.Name, color.FgCyan)
		if ps > 0 && pl > 0 {
			cells := term.Cprint(sal+" branch and ", color.Faint)
			cells = append(cells, ucs...)
			cells = append(cells, term.Cprint(" have diverged,", color.Faint)...)
			grid = append(grid, cells)
			cells = term.Cprint("and have ", color.Faint)
			cells = append(cells, term.Cprint(strconv.Itoa(ps), color.FgYellow)...)
			cells = append(cells, term.Cprint(" and ", color.Faint)...)
			cells = append(cells, term.Cprint(strconv.Itoa(pl), color.FgYellow)...)
			cells = append(cells, term.Cprint(" different commits each, respectively.", color.Faint)...)
			grid = append(grid, cells)
			grid = append(grid, term.Cprint("(\"pull\" to merge the remote branch into yours)", color.Faint))
		} else if pl > 0 && ps == 0 {
			cells := term.Cprint(sal+" branch is behind ", color.Faint)
			cells = append(cells, ucs...)
			cells = append(cells, term.Cprint(" by ", color.Faint)...)
			cells = append(cells, term.Cprint(strconv.Itoa(pl), color.FgYellow)...)
			cells = append(cells, term.Cprint(" commit(s).", color.Faint)...)
			grid = append(grid, cells)
			grid = append(grid, term.Cprint("(\"pull\" to update your local branch)", color.Faint))
		} else if ps > 0 && pl == 0 {
			cells := term.Cprint(sal+" branch is ahead of ", color.Faint)
			cells = append(cells, ucs...)
			cells = append(cells, term.Cprint(" by ", color.Faint)...)
			cells = append(cells, term.Cprint(strconv.Itoa(ps), color.FgYellow)...)
			cells = append(cells, term.Cprint(" commit(s).", color.Faint)...)
			grid = append(grid, cells)
			grid = append(grid, term.Cprint("(\"push\" to publish your local commit(s))", color.Faint))
		}
	}
	return grid
}

func workingTreeClean(b *git.Branch) [][]term.Cell {
	var grid [][]term.Cell
	grid = branchInfo(b, true)
	grid = append(grid, term.Cprint("Nothing to commit, working tree clean", color.Faint))
	return grid
}

// returns multiline so the return value will be a 2-d slice
func genHelp(pairs map[string]string) [][]term.Cell {
	var grid [][]term.Cell
	// sort keys alphabetically
	keys := make([]string, 0, len(pairs))
	for key := range pairs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		grid = append(grid, append(term.Cprint(fmt.Sprintf("%s: ", key), color.Faint),
			term.Cprint(fmt.Sprintf("%s", pairs[key]), color.FgYellow)...))
	}
	grid = append(grid, term.Cprint("", 0))
	grid = append(grid, term.Cprint("press any key to return.", color.Faint))
	return grid
}
