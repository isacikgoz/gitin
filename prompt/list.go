// This is a modified version of promptui's list. The original version can
// be found at https://github.com/manifoldco/promptui

package prompt

import (
	"fmt"
	"strings"

	"github.com/sahilm/fuzzy"
)

// Item is to create a simple interface for list items
type Item interface {
	String() string
}

type interfaceSource []Item

func (is interfaceSource) String(i int) string { return is[i].String() }

func (is interfaceSource) Len() int { return len(is) }

// NotFound is an index returned when no item was selected. This could
// happen due to a search without results.
const NotFound = -1

// List holds a collection of items that can be displayed with an N number of
// visible items. The list can be moved up, down by one item of time or an
// entire page (ie: visible size). It keeps track of the current selected item.
type List struct {
	items  []Item
	scope  []Item
	cursor int // cursor holds the index of the current selected item
	size   int // size is the number of visible options
	start  int
	find   string
}

// NewList creates and initializes a list of searchable items. The items attribute must be a slice type with a
// size greater than 0. Error will be returned if those two conditions are not met.
func NewList(items []Item, size int) (*List, error) {
	if size < 1 {
		return nil, fmt.Errorf("list size %d must be greater than 0", size)
	}

	return &List{size: size, items: items, scope: items}, nil
}

// Prev moves the visible list back one item. If the selected item is out of
// view, the new select item becomes the last visible item. If the list is
// already at the top, nothing happens.
func (l *List) Prev() {
	if l.cursor > 0 {
		l.cursor--
	}

	if l.start > l.cursor {
		l.start = l.cursor
	}
}

// Search allows the list to be filtered by a given term. The list must
// implement the searcher function signature for this functionality to work.
func (l *List) Search(term string) {
	term = strings.Trim(term, " ")
	l.cursor = 0
	l.start = 0
	l.find = term
	l.search(term)
}

// CancelSearch stops the current search and returns the list to its
// original order.
func (l *List) CancelSearch() {
	l.cursor = 0
	l.start = 0
	l.scope = l.items
}

func (l *List) search(term string) {
	if len(term) == 0 {
		l.scope = l.items
		return
	}
	results := fuzzy.FindFrom(term, interfaceSource(l.items))
	l.scope = make([]Item, 0)
	for _, r := range results {
		l.scope = append(l.scope, l.items[r.Index])
	}
}

// Start returns the current render start position of the list.
func (l *List) Start() int {
	return l.start
}

// SetStart sets the current scroll position. Values out of bounds will be
// clamped.
func (l *List) SetStart(i int) {
	if i < 0 {
		i = 0
	}
	if i > l.cursor {
		l.start = l.cursor
	} else {
		l.start = i
	}
}

// SetCursor sets the position of the cursor in the list. Values out of bounds
// will be clamped.
func (l *List) SetCursor(i int) {
	max := len(l.scope) - 1
	if i >= max {
		i = max
	}
	if i < 0 {
		i = 0
	}
	l.cursor = i

	if l.start > l.cursor {
		l.start = l.cursor
	} else if l.start+l.size <= l.cursor {
		l.start = l.cursor - l.size + 1
	}
}

// Next moves the visible list forward one item. If the selected item is out of
// view, the new select item becomes the first visible item. If the list is
// already at the bottom, nothing happens.
func (l *List) Next() {
	max := len(l.scope) - 1

	if l.cursor < max {
		l.cursor++
	}

	if l.start+l.size <= l.cursor {
		l.start = l.cursor - l.size + 1
	}
}

// PageUp moves the visible list backward by x items. Where x is the size of the
// visible items on the list. The selected item becomes the first visible item.
// If the list is already at the bottom, the selected item becomes the last
// visible item.
func (l *List) PageUp() {
	start := l.start - l.size
	if start < 0 {
		l.start = 0
	} else {
		l.start = start
	}

	cursor := l.start

	if cursor < l.cursor {
		l.cursor = cursor
	}
}

// PageDown moves the visible list forward by x items. Where x is the size of
// the visible items on the list. The selected item becomes the first visible
// item.
func (l *List) PageDown() {
	start := l.start + l.size
	max := len(l.scope) - l.size

	switch {
	case len(l.scope) < l.size:
		l.start = 0
	case start > max:
		l.start = max
	default:
		l.start = start
	}

	cursor := l.start

	if cursor == l.cursor {
		l.cursor = len(l.scope) - 1
	} else if cursor > l.cursor {
		l.cursor = cursor
	}
}

// CanPageDown returns whether a list can still PageDown().
func (l *List) CanPageDown() bool {
	max := len(l.scope)
	return l.start+l.size < max
}

// CanPageUp returns whether a list can still PageUp().
func (l *List) CanPageUp() bool {
	return l.start > 0
}

// Index returns the index of the item currently selected inside the searched list. If no item is selected,
// the NotFound (-1) index is returned.
func (l *List) Index() int {
	// defer recoverFromPanic()
	if len(l.scope) <= 0 {
		return 0
	}
	selected := l.scope[l.cursor]

	for i, item := range l.items {
		if item == selected {
			return i
		}
	}

	return NotFound
}

func recoverFromPanic() {
	if r := recover(); r != nil {
	}
}

// Items returns a slice equal to the size of the list with the current visible
// items and the index of the active item in this list.
func (l *List) Items() ([]Item, int) {
	var result []Item
	max := len(l.scope)
	end := l.start + l.size

	if end > max {
		end = max
	}

	active := NotFound

	for i, j := l.start, 0; i < end; i, j = i+1, j+1 {
		if l.cursor == i {
			active = j
		}

		result = append(result, l.scope[i])
	}

	return result, active
}
