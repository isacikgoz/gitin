package prompt

import (
	"fmt"
	"sort"

	"github.com/fatih/color"
	"github.com/isacikgoz/gitin/term"
)

func itemText(item interface{}, matches []int, selected bool) [][]term.Cell {
	var line []term.Cell
	text := fmt.Sprint(item)
	if selected {
		line = append(line, term.Cprint("> ", color.FgCyan)...)
	} else {
		line = append(line, term.Cprint("  ", color.FgWhite)...)
	}
	matched := make(map[int]bool, len(matches))
	for _, m := range matches {
		matched[m] = true // byte offsets in text
	}
	for i, r := range text {
		cell := term.Cell{Ch: r}
		if matched[i] {
			cell.Attr = []color.Attribute{color.Underline}
		}
		line = append(line, cell)
	}
	return [][]term.Cell{line}
}

// returns multiline so the return value will be a 2-d slice
func genHelp(pairs map[string]string) [][]term.Cell {
	var grid [][]term.Cell
	n := map[string][]string{}
	// sort keys alphabetically, sort by values
	keys := make([]string, 0, len(pairs))
	for k, v := range pairs {
		n[v] = append(n[v], k)
	}
	for k := range n {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		grid = append(grid, append(term.Cprint(fmt.Sprintf("%s: ", key), color.Faint),
			term.Cprint(n[key][0], color.FgYellow)...))
	}
	grid = append(grid, term.Cprint("", 0))
	grid = append(grid, term.Cprint("press any key to return.", color.Faint))
	return grid
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
