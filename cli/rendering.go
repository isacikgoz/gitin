package cli

import (
	"fmt"
	"strconv"

	"github.com/fatih/color"
	"github.com/isacikgoz/gitin/git"
	"github.com/isacikgoz/gitin/term"
)

func renderItem(item interface{}, matches []int, selected bool) [][]term.Cell {
	var line []term.Cell
	if selected {
		line = append(line, term.Cprint("> ", color.FgCyan)...)
	} else {
		line = append(line, term.Cprint("  ", color.FgWhite)...)
	}
	switch i := item.(type) {
	case *git.StatusEntry:
		attr := color.FgRed
		if i.Indexed() {
			attr = color.FgGreen
		}
		line = append(line, stautsText(i.StatusEntryString()[:1])...)
		line = append(line, highLightedText(matches, attr, i.String())...)
	case *git.Commit:
		line = append(line, stautsText(i.Hash[:7])...)
		line = append(line, highLightedText(matches, color.FgWhite, i.String())...)
	case *git.DiffDelta:
		line = append(line, stautsText(i.DeltaStatusString()[:1])...)
		line = append(line, highLightedText(matches, color.FgWhite, i.String())...)
	case *git.Branch:
		attr := color.FgWhite
		headIndicator := ""
		if i.Head {
			attr = color.FgGreen
			headIndicator = " *"
		} else if i.IsRemote() {
			attr = color.FgRed
		}
		line = append(line, highLightedText(matches, attr, i.String()+headIndicator)...)
	default:
		line = append(line, highLightedText(matches, color.FgWhite, fmt.Sprint(item))...)
	}
	return [][]term.Cell{line}
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

// highLightedText underlines the matched characters, matches are byte offsets in str
func highLightedText(matches []int, c color.Attribute, str string) []term.Cell {
	if len(matches) == 0 {
		return term.Cprint(str, c)
	}
	matched := make(map[int]bool, len(matches))
	for _, m := range matches {
		matched[m] = true
	}
	highlighted := make([]term.Cell, 0, len(str))
	for i, r := range str {
		attr := []color.Attribute{c}
		if matched[i] {
			attr = append(attr, color.Underline)
		}
		highlighted = append(highlighted, term.Cell{Ch: r, Attr: attr})
	}
	return highlighted
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
	if yours && b.Detached {
		cells := term.Cprint("HEAD detached at ", color.Faint)
		if len(b.Hash) >= 7 {
			cells = append(cells, term.Cprint(b.Hash[:7], color.FgYellow)...)
		}
		return append(grid, cells)
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
