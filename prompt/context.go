package prompt

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/isacikgoz/fuzzy"
)

type searchContext struct {
	ctx      context.Context
	cancel   func()
	buffer   []fuzzy.Match
	progress int32
	mx       sync.Mutex
}

func newSearchContext(c context.Context) *searchContext {
	ctx, cancel := context.WithCancel(c)
	return &searchContext{
		ctx:    ctx,
		cancel: cancel,
		buffer: make([]fuzzy.Match, 0),
		mx:     sync.Mutex{},
	}
}

func (c *searchContext) addBuffer(items ...fuzzy.Match) {
	c.mx.Lock()
	defer c.mx.Unlock()

	c.buffer = append(c.buffer, items...)
	sort.Stable(fuzzy.Sortable(c.buffer))
}

func (c *searchContext) getBuffer() []fuzzy.Match {
	return c.buffer
}

func (c *searchContext) clearBuffer() {
	c.mx.Lock()
	defer c.mx.Unlock()

	c.buffer = make([]fuzzy.Match, 0)
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

func (c *searchContext) startSearch(ctx context.Context) bool {
	c.ctx, c.cancel = context.WithCancel(ctx)
	return atomic.CompareAndSwapInt32(&c.progress, 0, 1)
}
