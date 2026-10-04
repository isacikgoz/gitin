package prompt

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// item returns a text where every 3rd item contains "x", every 9th "xy" and
// every 27th "xyz".
func item(i int) string {
	s := fmt.Sprintf("item-%06d ", i)
	if i%3 == 0 {
		s += "x"
	}
	if i%9 == 0 {
		s += "y"
	}
	if i%27 == 0 {
		s += "z"
	}
	return s
}

func feed(ch chan<- interface{}, from, to int) {
	for i := from; i < to; i++ {
		ch <- item(i)
	}
}

func waitWorkers(t *testing.T, l *AsyncList) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		l.workers.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("list workers did not finish")
	}
}

// assertScope checks that the results are exactly the items ending with
// suffix, each listed once.
func assertScope(t *testing.T, l *AsyncList, total int, suffix string) {
	t.Helper()
	l.mx.Lock()
	defer l.mx.Unlock()

	want := 0
	for i := 0; i < total; i++ {
		if strings.HasSuffix(item(i), suffix) {
			want++
		}
	}
	seen := make(map[interface{}]bool)
	for _, it := range l.scope {
		if !strings.HasSuffix(it.(string), suffix) {
			t.Fatalf("result %q does not match the search", it)
		}
		if seen[it] {
			t.Fatalf("result %q is listed twice", it)
		}
		seen[it] = true
	}
	if len(seen) != want {
		t.Fatalf("got %d results, want %d", len(seen), want)
	}
}

func TestAsyncListDropsResultsOfReplacedSearches(t *testing.T) {
	const total = 100000
	items := make(chan interface{})
	l, err := NewAsyncList(items, 5)
	if err != nil {
		t.Fatal(err)
	}
	feed(items, 0, total)
	close(items)
	waitWorkers(t, l)

	// typed one character at a time while earlier searches are still running
	for _, term := range []string{"x", "xy", "xyz"} {
		l.Search(term)
	}
	waitWorkers(t, l)

	assertScope(t, l, total, "xyz")
}

func TestAsyncListSearchesItemsLoadedLater(t *testing.T) {
	const total = 50000
	items := make(chan interface{})
	l, err := NewAsyncList(items, 5)
	if err != nil {
		t.Fatal(err)
	}
	feed(items, 0, total/2)
	l.Search("xyz")
	feed(items, total/2, total)
	close(items)
	waitWorkers(t, l)

	assertScope(t, l, total, "xyz")
}

// Nobody reads Update() while the prompt is busy handling a key, loading and
// searching must not wait for it.
func TestAsyncListDoesNotBlockOnUpdates(t *testing.T) {
	const total = 3 * loadBatchSize
	items := make(chan interface{})
	l, err := NewAsyncList(items, 5)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		feed(items, 0, total)
		close(items)
	}()
	for _, term := range []string{"x", "xy", "xyz", ""} {
		l.Search(term)
	}
	waitWorkers(t, l)

	if _, active := l.Items(); active != 0 {
		t.Fatalf("got active index %d, want 0", active)
	}
	l.mx.Lock()
	defer l.mx.Unlock()
	if len(l.scope) != total {
		t.Fatalf("got %d items, want %d", len(l.scope), total)
	}
}
