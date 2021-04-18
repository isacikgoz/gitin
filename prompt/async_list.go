package prompt

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/isacikgoz/fuzzy"
)

// AsyncList holds a collection of items that can be displayed with an N number of
// visible items. The list can be moved up, down by one item of time or an
// entire page (ie: visible size). It keeps track of the current selected item.
type AsyncList struct {
	itemsChan chan interface{}
	items     []interface{}
	scope     []interface{}
	buffer    []interface{}
	matches   sync.Map
	cursor    int // cursor holds the index of the current selected item
	size      int // size is the number of visible options
	start     int
	find      string
	mx        sync.Mutex
	update    chan struct{}
	ctx       *searchContext
}

type searchContext struct {
	ctx      context.Context
	cancel   func()
	buffer   []fuzzy.Match
	progress int32
}

func newSearchContext(c context.Context) *searchContext {
	ctx, cancel := context.WithCancel(c)
	return &searchContext{
		ctx:    ctx,
		cancel: cancel,
		buffer: make([]fuzzy.Match, 0),
	}
}

func (c *searchContext) addBuffer(items ...fuzzy.Match) {
	c.buffer = append(c.buffer, items...)
}

func (c *searchContext) getBuffer() []fuzzy.Match {
	return c.buffer
}

func (c *searchContext) clearBuffer() {
	c.buffer = make([]fuzzy.Match, 0)
	return
}

func (c *searchContext) searchInProgress() bool {
	return atomic.LoadInt32(&c.progress) != 0
}

func (c *searchContext) stopSearch() {
	if atomic.LoadInt32(&c.progress) == 0 {
		return
	}

	c.cancel()
	c.clearBuffer()
	return
}

func (c *searchContext) startSearch() bool {
	c.ctx, c.cancel = context.WithCancel(context.Background())
	return atomic.CompareAndSwapInt32(&c.progress, 0, 1)
}

// NewAsyncList creates and initializes a list of searchable items. The items attribute must be a slice type.
func NewAsyncList(items chan interface{}, size int) (*AsyncList, error) {
	if size < 1 {
		return nil, fmt.Errorf("list size %d must be greater than 0", size)
	}

	if items == nil || reflect.TypeOf(items).Kind() != reflect.Chan {
		return nil, fmt.Errorf("items %v is not a chan", items)
	}

	is := make([]interface{}, 0)
	list := &AsyncList{
		size:      size,
		items:     is,
		itemsChan: items,
		scope:     is,
		mx:        sync.Mutex{},
		update:    make(chan struct{}),
		buffer:    make([]interface{}, 0),
		ctx:       newSearchContext(context.Background()),
	}

	go func() {
		var flush int
		for val := range items {
			list.addBuffer(val)
			if flush < 4096 {
				flush++
				continue
			}
			list.flushBuffer()
			flush = 0
		}
		list.flushBuffer()
	}()

	return list, nil
}

func (l *AsyncList) flushBuffer() {
	l.mx.Lock()
	defer l.mx.Unlock()

	if len(l.buffer) == 0 {
		return
	}

	l.items = append(l.items, l.buffer...)
	l.scope = append(l.scope, l.buffer...)

	// // update for the first iteration of flushing the buffer
	// // then, a user input will trigger a re-rendering operation
	// if l.update != nil {
	l.update <- struct{}{}
	// }
	// l.update = nil

	l.buffer = make([]interface{}, 0)
}

func (l *AsyncList) addBuffer(items ...interface{}) {
	l.buffer = append(l.buffer, items...)
}

// Prev moves the visible list back one item.
func (l *AsyncList) Prev() {
	if l.cursor > 0 {
		l.cursor--
	}

	if l.start > l.cursor {
		l.start = l.cursor
	}
}

// Search allows the list to be filtered by a given term.
func (l *AsyncList) Search(term string) {
	l.mx.Lock()
	defer l.mx.Unlock()

	term = strings.Trim(term, " ")
	l.cursor = 0
	l.start = 0
	l.find = term
	l.search(term)
}

// CancelSearch stops the current search and returns the list to its original order.
func (l *AsyncList) CancelSearch() {
	l.cursor = 0
	l.start = 0
	l.scope = l.items
}

func (l *AsyncList) flushToScope(ctx *searchContext, fireUpdate bool) {
	defer ctx.clearBuffer()

	sort.Stable(fuzzy.Sortable(ctx.buffer))
	for _, match := range ctx.buffer {
		item := l.items[match.Index]
		l.scope = append(l.scope, item)
		l.matches.Store(item, match.MatchedIndexes)
	}
	if fireUpdate && l.update != nil {
		l.update <- struct{}{}
	}
}

func (l *AsyncList) search(term string) {
	if len(term) == 0 {
		l.scope = l.items
		return
	}

	l.ctx.stopSearch()
	l.matches = sync.Map{}
	l.scope = make([]interface{}, 0)

	l.ctx.startSearch()
	results := fuzzy.FindFrom(context.Background(), term, interfaceSource(l.items))

	go func() {
		var flush int
		var done bool
		for result := range results {
			if !l.ctx.searchInProgress() {
				break
			}
			l.ctx.addBuffer(result)

			if !done && flush == l.size {
				l.flushToScope(l.ctx, true)
				done = true
				continue
			}

			if flush < 16384 {
				flush++
				continue
			}

			l.flushToScope(l.ctx, false)
			flush = 0
		}
		l.flushToScope(l.ctx, true)
		l.ctx.clearBuffer()
	}()
}

// Start returns the current render start position of the list.
func (l *AsyncList) Start() int {
	return l.start
}

// SetStart sets the current scroll position. Values out of bounds will be clamped.
func (l *AsyncList) SetStart(i int) {
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
	max := len(l.scope)
	return l.start+l.size < max
}

// CanPageUp returns whether a list can still PageUp().
func (l *AsyncList) CanPageUp() bool {
	return l.start > 0
}

// Index returns the index of the item currently selected inside the searched list.
func (l *AsyncList) Index() int {
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

func (l *AsyncList) Size() int {
	return l.size
}

func (l *AsyncList) Cursor() int {
	return l.cursor
}

func (l *AsyncList) Matches(key interface{}) []int {
	v, ok := l.matches.Load(key)
	if !ok {
		return make([]int, 0)
	}
	return v.([]int)
}

func (l *AsyncList) Update() chan struct{} {
	return l.update
}
