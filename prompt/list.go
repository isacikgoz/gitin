package prompt

type List interface {
	// Next moves the visible list forward one item
	Next()

	// Prev moves the visible list back one item.
	Prev()

	// PageUp moves the visible list backward by x items. Where x is the size of the
	// visible items on the list
	PageUp()

	// PageDown moves the visible list forward by x items. Where x is the size of
	// the visible items on the list
	PageDown()

	// CanPageDown returns whether a list can still PageDown().
	CanPageDown() bool

	// CanPageUp returns whether a list can still PageUp()
	CanPageUp() bool

	// Search allows the list to be filtered by a given term.
	Search(term string)

	// CancelSearch stops the current search and returns the list to its original order.
	CancelSearch()

	// Start returns the current render start position of the list.
	Start() int

	// SetStart sets the current scroll position. Values out of bounds will be clamped.
	SetStart(i int)

	// SetCursor sets the position of the cursor in the list. Values out of bounds will
	// be clamped.
	SetCursor(i int)

	// Index returns the index of the item currently selected inside the searched list
	Index() int

	// Items returns a slice equal to the size of the list with the current visible
	// items and the index of the active item in this list.
	Items() ([]interface{}, int)

	// Matches returns the matched items against a search term
	Matches(key interface{}) []int

	// Cursor is the current cursor position
	Cursor() int

	// Size is the number of items to be displayed
	Size() int

	Update() chan struct{}
}
