package cli

import (
	"fmt"
	"strings"
	"sync"

	"github.com/isacikgoz/promptui/list"
	"github.com/sahilm/fuzzy"
)

type interfaceSource []*interface{}

func (is interfaceSource) String(i int) string { return fmt.Sprint(*is[i]) }

func (is interfaceSource) Len() int { return len(is) }

func combinedSearch(in []*interface{}, term string) []*interface{} {
	scope := interfaceSource(in)
	var wg sync.WaitGroup
	filter := strings.ToLower(term)
	if filter == "" {
		return scope
	}
	if strings.HasSuffix(filter, "!") {
		filter = filter[:len(filter)-1]
	}
	eMatches := make([]*interface{}, 0)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, o := range scope {
			if strings.Contains(strings.ToLower(fmt.Sprint(*o)), filter) {
				eMatches = append(eMatches, o)
			}
		}
	}()
	if strings.HasSuffix(term, "!") {
		wg.Wait()
		return eMatches
	}
	fMatches := make([]*interface{}, 0)
	wg.Add(1)
	go func() {
		defer wg.Done()
		fMatches = fuzzySearch(in, term)
		eMatches = append(eMatches, fMatches...)
	}()

	wg.Wait()
	return removeDuplicates(eMatches)
}

func fuzzySearch(in []*interface{}, term string) []*interface{} {
	scope := interfaceSource(in)
	var wg sync.WaitGroup
	filter := strings.ToLower(term)
	if filter == "" {
		return scope
	}
	fMatches := make([]*interface{}, 0)
	wg.Add(1)
	go func() {
		defer wg.Done()
		matches := fuzzy.FindFrom(filter, scope)
		for _, m := range matches {
			fMatches = append(fMatches, in[m.Index])
		}
	}()

	wg.Wait()
	return fMatches
}

func basicSearch(scope []*interface{}, term string) []*interface{} {
	matches := make([]*interface{}, 0)
	for _, o := range scope {
		if strings.Contains(strings.ToLower(fmt.Sprint(*o)), strings.ToLower(term)) {
			matches = append(matches, o)
		}
	}
	return matches
}

// *interface{} shall implement String() interface
// removes duplicate entries from prompt.Suggest slice
func removeDuplicates(elements []*interface{}) []*interface{} {
	// Use map to record duplicates as we find them.
	encountered := map[*interface{}]bool{}
	result := make([]*interface{}, 0)

	for v := range elements {
		if encountered[elements[v]] == true {
			// Do not add duplicate.
		} else {
			// Record this element as an encountered element.
			encountered[elements[v]] = true
			// Append to result slice.
			result = append(result, elements[v])
		}
	}
	// Return the new slice.
	return result
}

func finderFunc(option string) list.Searcher {
	switch option {
	case "combined":
		return combinedSearch
	case "basic":
		return basicSearch
	default:
		return fuzzySearch
	}
}
