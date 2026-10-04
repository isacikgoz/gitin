package prompt

import (
	"fmt"
	"testing"
)

func numbered(n int) []string {
	items := make([]string, n)
	for i := range items {
		items[i] = fmt.Sprintf("item-%d", i)
	}
	return items
}

// newLists returns a SyncList and a fully loaded AsyncList of the same items
func newLists(t *testing.T, items []string, size int) map[string]List {
	t.Helper()
	syncList, err := NewList(items, size)
	if err != nil {
		t.Fatal(err)
	}
	ch := make(chan interface{}, len(items))
	for _, item := range items {
		ch <- item
	}
	close(ch)
	asyncList, err := NewAsyncList(ch, size)
	if err != nil {
		t.Fatal(err)
	}
	waitWorkers(t, asyncList)
	return map[string]List{"SyncList": syncList, "AsyncList": asyncList}
}

// view describes the visible items and the active one, e.g. "[item-2 >item-3 item-4]"
func view(l List) string {
	items, active := l.Items()
	s := "["
	for i, item := range items {
		if i > 0 {
			s += " "
		}
		if i == active {
			s += ">"
		}
		s += fmt.Sprint(item)
	}
	return s + "]"
}

func TestListNavigation(t *testing.T) {
	for name, l := range newLists(t, numbered(10), 3) {
		t.Run(name, func(t *testing.T) {
			steps := []struct {
				name string
				move func()
				want string
			}{
				{"start", func() {}, "[>item-0 item-1 item-2]"},
				{"prev at top", l.Prev, "[>item-0 item-1 item-2]"},
				{"next x4", func() {
					for range 4 {
						l.Next()
					}
				}, "[item-2 item-3 >item-4]"},
				{"prev", l.Prev, "[item-2 >item-3 item-4]"},
				{"page down", l.PageDown, "[>item-5 item-6 item-7]"},
				{"page down to last page", l.PageDown, "[>item-7 item-8 item-9]"},
				{"page down on last page", l.PageDown, "[item-7 item-8 >item-9]"},
				{"next at bottom", l.Next, "[item-7 item-8 >item-9]"},
				{"page up", l.PageUp, "[>item-4 item-5 item-6]"},
				{"cursor past the end", func() { l.SetCursor(100) }, "[item-7 item-8 >item-9]"},
				{"cursor before the start", func() { l.SetCursor(-5) }, "[>item-0 item-1 item-2]"},
				{"start after the cursor", func() { l.SetStart(5) }, "[>item-0 item-1 item-2]"},
				{"page up at top", l.PageUp, "[>item-0 item-1 item-2]"},
			}
			for _, step := range steps {
				step.move()
				if got := view(l); got != step.want {
					t.Fatalf("%s: got %s, want %s", step.name, got, step.want)
				}
			}
			if l.Size() != 3 || l.Cursor() != 0 || l.Start() != 0 {
				t.Fatalf("got size %d cursor %d start %d", l.Size(), l.Cursor(), l.Start())
			}
			if !l.CanPageDown() || l.CanPageUp() {
				t.Fatal("can't page down or can page up at the top")
			}
			l.SetCursor(9)
			if l.CanPageDown() || !l.CanPageUp() {
				t.Fatal("can page down or can't page up at the bottom")
			}
			l.SetCursor(4)
			l.SetStart(3)
			if got := view(l); got != "[item-3 >item-4 item-5]" {
				t.Fatalf("set start: got %s", got)
			}
		})
	}
}

func TestListSearch(t *testing.T) {
	items := []string{"apple pie", "banana split", "cherry tart", "apple crumble", "Ünïcode apple"}
	for name, l := range newLists(t, items, 5) {
		t.Run(name, func(t *testing.T) {
			search := func(term string) {
				l.Search(term)
				if a, ok := l.(*AsyncList); ok {
					waitWorkers(t, a)
				}
			}

			search(" apple ")
			items, active := l.Items()
			if len(items) != 3 || active != 0 {
				t.Fatalf("got %v active %d, want the 3 apples", items, active)
			}
			for _, item := range items {
				if len(l.Matches(item)) != len("apple") {
					t.Fatalf("%s: got matches %v", item, l.Matches(item))
				}
			}
			// matches are byte offsets
			if m := l.Matches("Ünïcode apple"); m[0] != len("Ünïcode ") {
				t.Fatalf("got matches %v in a non-ASCII text", m)
			}
			// Index is the position of the selected match in all items
			l.Next()
			want := map[interface{}]int{"apple pie": 0, "apple crumble": 3, "Ünïcode apple": 4}[items[1]]
			if l.Index() != want {
				t.Fatalf("got index %d, want %d for %s", l.Index(), want, items[1])
			}

			search("zzz")
			if items, active := l.Items(); len(items) != 0 || active != NotFound || l.Index() != 0 {
				t.Fatalf("got %v active %d index %d for no matches", items, active, l.Index())
			}

			l.CancelSearch()
			if got := view(l); got != "[>apple pie banana split cherry tart apple crumble Ünïcode apple]" {
				t.Fatalf("after cancel: got %s", got)
			}
			if len(l.Matches("apple pie")) != 0 {
				t.Fatal("matches are kept after the search is cancelled")
			}
			if l.Index() != 0 {
				t.Fatalf("got index %d", l.Index())
			}
		})
	}
}

func TestNewListErrors(t *testing.T) {
	if _, err := NewList([]string{"a"}, 0); err == nil {
		t.Error("NewList accepts size 0")
	}
	if _, err := NewList("not a slice", 3); err == nil {
		t.Error("NewList accepts a string")
	}
	if _, err := NewList(nil, 3); err == nil {
		t.Error("NewList accepts nil")
	}
	if _, err := NewAsyncList(make(chan interface{}), 0); err == nil {
		t.Error("NewAsyncList accepts size 0")
	}
	if _, err := NewAsyncList(nil, 3); err == nil {
		t.Error("NewAsyncList accepts a nil channel")
	}
}

func TestListUpdate(t *testing.T) {
	l, err := NewList([]string{"a"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if l.Update() != nil {
		t.Fatal("SyncList has an update channel")
	}

	ch := make(chan interface{})
	a, err := NewAsyncList(ch, 1)
	if err != nil {
		t.Fatal(err)
	}
	ch <- "first"
	select {
	case <-a.Update():
	case <-timeout():
		t.Fatal("no update after the first page was loaded")
	}
	close(ch)
	waitWorkers(t, a)
}
