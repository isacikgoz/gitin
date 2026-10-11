package e2e

import (
	"flag"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// The terminal of a session is an emulator: the screen is what a user sees,
// characters and colors, after gitin's escape sequences moved the cursor,
// cleared lines and redrew them.

var update = flag.Bool("update", false, "write the screens matchScreen compares to testdata/screens")

const (
	rows, cols = 40, 120
	// settle is how long the output pauses before a screen counts as drawn
	settle = 50 * time.Millisecond
)

// text returns the characters of a line of cells, without trailing spaces
func text(cells func(x int) *uv.Cell) string {
	var b strings.Builder
	for x := 0; x < cols; x++ {
		switch c := cells(x); {
		case c == nil:
			b.WriteByte(' ')
		case c.Width == 0: // the second half of a wide character
		default:
			b.WriteString(c.Content)
		}
	}
	return strings.TrimRight(b.String(), " ")
}

// screenRows returns the rows of the screen up to the last one with text,
// call it with s.mx held
func (s *session) screenRows() []string {
	var lines []string
	for y := 0; y < rows; y++ {
		lines = append(lines, text(func(x int) *uv.Cell { return s.term.CellAt(x, y) }))
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// screen returns the text on the screen
func (s *session) screen() string {
	s.mx.Lock()
	defer s.mx.Unlock()
	return strings.Join(s.screenRows(), "\n")
}

// output returns the text a user can scroll back to and the screen
func (s *session) output() string {
	s.mx.Lock()
	defer s.mx.Unlock()
	var lines []string
	for y := 0; y < s.term.ScrollbackLen(); y++ {
		lines = append(lines, text(func(x int) *uv.Cell { return s.term.ScrollbackCellAt(x, y) }))
	}
	return strings.Join(append(lines, s.screenRows()...), "\n")
}

// styledScreen describes the screen for matchScreen: its text, the styles of
// the text row by row and the cursor. Times, durations and temporary
// directories are masked, they differ from run to run.
func (s *session) styledScreen() string {
	s.mx.Lock()
	defer s.mx.Unlock()
	var b strings.Builder
	screen := s.screenRows()
	for _, row := range screen {
		b.WriteString(mask(row) + "\n")
	}
	b.WriteString("-- styles --\n")
	for y := range screen {
		if spans := styleSpans(func(x int) *uv.Cell { return s.term.CellAt(x, y) }); spans != "" {
			fmt.Fprintf(&b, "%d %s\n", y+1, spans)
		}
	}
	b.WriteString("-- cursor --\n")
	if s.hidden {
		b.WriteString("hidden\n")
	} else {
		pos := s.term.CursorPosition()
		fmt.Fprintf(&b, "row %d, column %d\n", pos.Y+1, pos.X+1)
	}
	return b.String()
}

// styleSpans lists the styled text of a row, e.g. `bold cyan "main"`
func styleSpans(cells func(x int) *uv.Cell) string {
	var spans []string
	var style uv.Style
	var span strings.Builder
	flush := func() {
		if span.Len() > 0 && !style.IsZero() {
			spans = append(spans, fmt.Sprintf("%s %q", describe(style), mask(span.String())))
		}
		span.Reset()
	}
	for x := 0; x < cols; x++ {
		c := cells(x)
		if c == nil {
			c = &uv.EmptyCell
		}
		if c.Width == 0 {
			continue
		}
		if !c.Style.Equal(&style) {
			flush()
			style = c.Style
		}
		span.WriteString(c.Content)
	}
	flush()
	return strings.Join(spans, ", ")
}

// describe names the attributes and colors of a style
func describe(s uv.Style) string {
	var words []string
	for _, attr := range []struct {
		bit  uint8
		name string
	}{
		{uv.AttrBold, "bold"}, {uv.AttrFaint, "faint"}, {uv.AttrItalic, "italic"}, {uv.AttrBlink, "blink"},
		{uv.AttrRapidBlink, "rapid blink"}, {uv.AttrReverse, "reverse"}, {uv.AttrConceal, "conceal"}, {uv.AttrStrikethrough, "strikethrough"},
	} {
		if s.Attrs&attr.bit != 0 {
			words = append(words, attr.name)
		}
	}
	if s.Underline != uv.UnderlineNone {
		words = append(words, "underline")
	}
	if s.Fg != nil {
		words = append(words, colorName(s.Fg))
	}
	if s.Bg != nil {
		words = append(words, "on "+colorName(s.Bg))
	}
	return strings.Join(words, " ")
}

var basicColors = []string{"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white"}

func colorName(c color.Color) string {
	var index int
	switch c := c.(type) {
	case ansi.BasicColor:
		index = int(c)
	case ansi.IndexedColor:
		index = int(c)
	default:
		r, g, b, _ := c.RGBA()
		return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
	}
	switch {
	case index < 8:
		return basicColors[index]
	case index < 16:
		return "bright " + basicColors[index-8]
	}
	return fmt.Sprintf("color %d", index)
}

var (
	// the durations gitin shows, e.g. "passed in 1.2s"
	durations = regexp.MustCompile(`\b(after|in) [0-9][0-9.hmµn]*s\b`)
	// the relative times gitin shows, e.g. "about 2 hours ago"
	relativeTimes = regexp.MustCompile(`\b(less than a minute|1 minute|\d+ minutes|about \d+ hours?|1 day|\d+ days|about [12] months?|\d+ months|(about|over|almost) \d+ years?) (ago|from now)\b`)
	// the directories of t.TempDir, which differ from run to run
	tempDirs = tempDirPattern()
)

func tempDirPattern() *regexp.Regexp {
	root := filepath.Clean(os.TempDir())
	roots := []string{regexp.QuoteMeta(root)}
	// e.g. /var/folders/... on macOS is /private/var/folders/...
	if real, err := filepath.EvalSymlinks(root); err == nil && real != root {
		roots = append(roots, regexp.QuoteMeta(real))
	}
	return regexp.MustCompile(`(` + strings.Join(roots, "|") + `)/[^/\s]+`)
}

func mask(s string) string {
	s = tempDirs.ReplaceAllString(s, "<tmp>")
	s = durations.ReplaceAllString(s, "$1 <duration>")
	return relativeTimes.ReplaceAllString(s, "<time>")
}

// matchScreen waits until the screen is the one in
// testdata/screens/<test>/<name>.txt. With -update, it writes the screen
// there once gitin stopped drawing.
func (s *session) matchScreen(name string) {
	s.t.Helper()
	path := filepath.Join("testdata", "screens", s.t.Name(), name+".txt")
	if *update {
		s.waitUntil("a drawn screen", func(string) bool { return true })
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			s.t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(s.styledScreen()), 0o644); err != nil {
			s.t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		s.t.Fatalf("%v, write it with go test ./e2e -run '^%s$' -update", err, strings.Split(s.t.Name(), "/")[0])
	}
	deadline := time.Now().Add(timeout)
	for {
		got := s.styledScreen()
		if got == string(want) {
			return
		}
		if time.Now().After(deadline) {
			s.t.Fatalf("the screen differs from %s:\n%s\nrun go test ./e2e -run '^%s$' -update and review the change",
				path, diff(string(want), got), strings.Split(s.t.Name(), "/")[0])
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// diff shows want and got line by line, a > marks the lines that differ
func diff(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	var b strings.Builder
	for i := 0; i < max(len(w), len(g)); i++ {
		var wl, gl string
		if i < len(w) {
			wl = w[i]
		}
		if i < len(g) {
			gl = g[i]
		}
		if wl == gl {
			fmt.Fprintf(&b, "   %s\n", gl)
			continue
		}
		fmt.Fprintf(&b, "> want: %s\n> got:  %s\n", wl, gl)
	}
	return b.String()
}

// styleOf describes the style of the last text on the screen, "" if the
// screen doesn't show it and "mixed" if its characters differ in style
func (s *session) styleOf(text string) string {
	s.mx.Lock()
	defer s.mx.Unlock()
	for y := rows - 1; y >= 0; y-- {
		var row strings.Builder
		var starts []int // the byte offset of each cell in row
		var cells []*uv.Cell
		for x := 0; x < cols; x++ {
			c := s.term.CellAt(x, y)
			if c == nil {
				c = &uv.EmptyCell
			}
			if c.Width == 0 {
				continue
			}
			starts = append(starts, row.Len())
			cells = append(cells, c)
			row.WriteString(c.Content)
		}
		i := strings.LastIndex(row.String(), text)
		if i < 0 {
			continue
		}
		style := ""
		for j, c := range cells {
			if starts[j] < i || starts[j] >= i+len(text) {
				continue
			}
			if d := describe(c.Style); style == "" {
				style = d
			} else if d != style {
				return "mixed"
			}
		}
		return style
	}
	return ""
}

// waitStyle waits until gitin drew the last text on the screen in style,
// e.g. "green"
func (s *session) waitStyle(text, style string) {
	s.t.Helper()
	s.waitUntil(fmt.Sprintf("%q in %s", text, style), func(string) bool {
		return s.styleOf(text) == style
	})
}
