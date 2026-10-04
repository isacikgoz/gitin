package prompt

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/isacikgoz/fuzzy"
)

const (
	// loadBatchSize is the number of loaded items buffered before they are
	// added to the list. The first batch is only as large as the visible size
	// so the first screen shows up as early as possible.
	loadBatchSize = 4096
	// searchBatchSize is the number of matches collected before they are
	// added to the search results.
	searchBatchSize = 16384
)

// AsyncList holds a collection of items that can be displayed with an N number of
// visible items. The list can be moved up, down by one item of time or an
// entire page (ie: visible size). It keeps track of the current selected item.
// Items are loaded from a channel and searched in the background, every
// method is safe for concurrent use.
type AsyncList struct {
	mx      sync.Mutex
	items   []interface{}
	texts   []string // texts[i] is the searchable text of items[i]
	scope   []interface{}
	matches map[interface{}][]int
	cursor  int // cursor holds the index of the current selected item
	size    int // size is the number of visible options
	start   int

	loading bool
	grown   chan struct{} // closed when items are appended or loading ends

	term         string
	generation   int // incremented by every search, results of older searches are dropped
	cancelSearch context.CancelFunc

	update  chan struct{}
	workers sync.WaitGroup // loader and search goroutines
}

// NewAsyncList creates and initializes a list of searchable items loaded from the given channel.
func NewAsyncList(items <-chan interface{}, size int) (*AsyncList, error) {
	if size < 1 {
		return nil, fmt.Errorf("list size %d must be greater than 0", size)
	}
	if items == nil {
		return nil, fmt.Errorf("items channel is nil")
	}

	list := &AsyncList{
		size:         size,
		matches:      make(map[interface{}][]int),
		loading:      true,
		grown:        make(chan struct{}),
		cancelSearch: func() {},
		update:       make(chan struct{}, 1),
	}
	list.workers.Add(1)
	go list.load(items)

	return list, nil
}

func (l *AsyncList) load(items <-chan interface{}) {
	defer l.workers.Done()

	limit := l.size
	batch := make([]interface{}, 0, limit)
	for item := range items {
		batch = append(batch, item)
		if len(batch) < limit {
			continue
		}
		l.appendItems(batch, false)
		limit = loadBatchSize
		batch = make([]interface{}, 0, limit)
	}
	l.appendItems(batch, true)
}

func (l *AsyncList) appendItems(batch []interface{}, done bool) {
	texts := make([]string, len(batch))
	for i, item := range batch {
		texts[i] = fmt.Sprint(item)
	}

	l.mx.Lock()
	l.items = append(l.items, batch...)
	l.texts = append(l.texts, texts...)
	if l.term == "" {
		l.scope = l.items
	}
	if done {
		l.loading = false
	}
	close(l.grown)
	l.grown = make(chan struct{})
	l.mx.Unlock()

	l.notify()
}

// notify asks the prompt to render again without ever blocking. Pending
// notifications are coalesced since a single render shows the latest state.
func (l *AsyncList) notify() {
	select {
	case l.update <- struct{}{}:
	default:
	}
}

// Prev moves the visible list back one item.
func (l *AsyncList) Prev() {
	l.mx.Lock()
	defer l.mx.Unlock()

	if l.cursor > 0 {
		l.cursor--
	}

	if l.start > l.cursor {
		l.start = l.cursor
	}
}

// Search allows the list to be filtered by a given term. The results are
// collected in the background, items loaded later are searched as well.
func (l *AsyncList) Search(term string) {
	term = strings.Trim(term, " ")

	l.mx.Lock()
	defer l.mx.Unlock()

	l.cancelSearch()
	l.cancelSearch = func() {}
	l.generation++
	l.cursor = 0
	l.start = 0
	l.term = term
	l.matches = make(map[interface{}][]int)

	if term == "" {
		l.scope = l.items
		return
	}

	l.scope = make([]interface{}, 0)
	ctx, cancel := context.WithCancel(context.Background())
	l.cancelSearch = cancel
	l.workers.Add(1)
	go l.search(ctx, l.generation, term)
}

// CancelSearch stops the current search and returns the list to its original order.
func (l *AsyncList) CancelSearch() {
	l.Search("")
}

func (l *AsyncList) search(ctx context.Context, generation int, term string) {
	defer l.workers.Done()

	next := 0
	for {
		l.mx.Lock()
		if l.generation != generation {
			l.mx.Unlock()
			return
		}
		items, texts := l.items[next:], l.texts[next:]
		loading, grown := l.loading, l.grown
		l.mx.Unlock()

		if len(items) == 0 {
			if !loading {
				return
			}
			select {
			case <-grown:
				continue
			case <-ctx.Done():
				return
			}
		}
		if !l.searchChunk(ctx, generation, term, items, texts) {
			return
		}
		next += len(items)
	}
}

// searchChunk adds the matching items of a chunk to the search results. It
// returns false if the search was replaced by a newer one.
func (l *AsyncList) searchChunk(ctx context.Context, generation int, term string, items []interface{}, texts []string) bool {
	results := fuzzy.Find(ctx, term, texts)
	// fuzzy blocks on sending a match even after ctx is cancelled
	defer func() {
		for range results {
		}
	}()

	limit := l.size
	batch := make([]fuzzy.Match, 0, limit)
	flush := func() bool {
		sort.Stable(fuzzy.Sortable(batch))

		l.mx.Lock()
		current := l.generation == generation
		if current {
			for _, match := range batch {
				item := items[match.Index]
				l.scope = append(l.scope, item)
				l.matches[item] = match.MatchedIndexes
			}
		}
		l.mx.Unlock()

		batch = batch[:0]
		if current {
			l.notify()
		}
		return current
	}

	for match := range results {
		batch = append(batch, match)
		if len(batch) < limit {
			continue
		}
		if !flush() {
			return false
		}
		limit = searchBatchSize
	}
	return flush()
}

// Start returns the current render start position of the list.
func (l *AsyncList) Start() int {
	l.mx.Lock()
	defer l.mx.Unlock()

	return l.start
}

// SetStart sets the current scroll position. Values out of bounds will be clamped.
func (l *AsyncList) SetStart(i int) {
	l.mx.Lock()
	defer l.mx.Unlock()

	if i < 0 {
		i = 0
	}
	if i > l.cursor {
		l.start = l.cursor
	} else {
		l.start = i
	}
}

// SetCursor sets the position of the cursor in the list. Values out of bounds will
// be clamped.
func (l *AsyncList) SetCursor(i int) {
	l.mx.Lock()
	defer l.mx.Unlock()

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

// Next moves the visible list forward one item.
func (l *AsyncList) Next() {
	l.mx.Lock()
	defer l.mx.Unlock()

	max := len(l.scope) - 1

	if l.cursor < max {
		l.cursor++
	}

	if l.start+l.size <= l.cursor {
		l.start = l.cursor - l.size + 1
	}
}

// PageUp moves the visible list backward by x items. Where x is the size of the
// visible items on the list.
func (l *AsyncList) PageUp() {
	l.mx.Lock()
	defer l.mx.Unlock()

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
// the visible items on the list.
func (l *AsyncList) PageDown() {
	l.mx.Lock()
	defer l.mx.Unlock()

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
func (l *AsyncList) CanPageDown() bool {
	l.mx.Lock()
	defer l.mx.Unlock()

	return l.start+l.size < len(l.scope)
}

// CanPageUp returns whether a list can still PageUp().
func (l *AsyncList) CanPageUp() bool {
	l.mx.Lock()
	defer l.mx.Unlock()

	return l.start > 0
}

// Index returns the index of the item currently selected inside the searched list.
func (l *AsyncList) Index() int {
	l.mx.Lock()
	defer l.mx.Unlock()

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

// Items returns a slice equal to the size of the list with the current visible
// items and the index of the active item in this list.
func (l *AsyncList) Items() ([]interface{}, int) {
	l.mx.Lock()
	defer l.mx.Unlock()

	var result []interface{}
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

// Size is the number of items to be displayed
func (l *AsyncList) Size() int {
	return l.size
}

// Cursor is the current cursor position
func (l *AsyncList) Cursor() int {
	l.mx.Lock()
	defer l.mx.Unlock()

	return l.cursor
}

// Matches returns the matched character offsets of an item for the current search
func (l *AsyncList) Matches(key interface{}) []int {
	l.mx.Lock()
	defer l.mx.Unlock()

	return l.matches[key]
}

// Update returns a channel that receives a value whenever the list changes in the background
func (l *AsyncList) Update() chan struct{} {
	return l.update
}
