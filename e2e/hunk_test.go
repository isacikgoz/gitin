package e2e

import (
	"fmt"
	"strings"
	"testing"

	"github.com/isacikgoz/gitin/internal/gittest"
)

// The hunk editor's terminal library used to leave a goroutine behind every
// run that stole key presses from the next run, after a few files the editor
// stopped responding.
func TestStatusHunkStagingManyFiles(t *testing.T) {
	const files = 8
	dir := gittest.NewRepo(t)
	content := map[string]string{}
	for i := range files {
		content[fmt.Sprintf("f%d.txt", i)] = "a\n"
	}
	gittest.Commit(t, dir, "base", content)
	for i := range files {
		gittest.WriteFile(t, dir, fmt.Sprintf("f%d.txt", i), "b\n")
	}

	s := start(t, dir, []string{"status"})
	s.waitFrame("Files", 0, "f0.txt")
	for i := range files {
		file := fmt.Sprintf("f%d.txt", i)
		m := s.mark()
		s.send("p")
		s.waitText(m, "+b")
		m = s.mark()
		s.send(" ", "q")
		s.waitFrame("Files", m, "> [M] "+file)
		if !strings.Contains(shortStatus(t, dir), "M  "+file) {
			t.Fatalf("file %d was not staged:\n%s", i, shortStatus(t, dir))
		}
		s.send(down) // from the staged file to the next one
	}
}
