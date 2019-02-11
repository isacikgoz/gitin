package editor

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/jroimartin/gocui"
	"github.com/waigani/diffparser"
)

const (
	headerLength = 4
)

var (
	green   = color.New(color.FgGreen)
	red     = color.New(color.FgRed)
	cyan    = color.New(color.FgCyan)
	hiWhite = color.New(color.FgHiWhite)
	bold    = color.New(color.Bold)
	whitebg = color.New(color.BgWhite)
	black   = color.New(color.FgBlack)
)

type Editor struct {
	g           *gocui.Gui
	KeyBindings []*KeyBinding
	State       *EditorState
}

type EditorState struct {
	File        *diffparser.DiffFile
	Patches     []string
	editorHunks []*editorHunk
}

type editorHunk struct {
	selected bool
	staged   bool
	hunk     *diffparser.DiffHunk
}

func NewEditor(file *diffparser.DiffFile) (*Editor, error) {
	eHunks := make([]*editorHunk, 0)
	for _, hunk := range file.Hunks {
		eHunks = append(eHunks, &editorHunk{
			selected: false,
			staged:   false,
			hunk:     hunk,
		})
	}
	eHunks[0].selected = true
	initialState := &EditorState{
		File:        file,
		editorHunks: eHunks,
	}
	e := &Editor{
		State: initialState,
	}
	if err := e.generateKeybindings(); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *Editor) Run() ([]string, error) {
	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		return nil, err
	}
	defer g.Close()

	e.g = g
	g.Cursor = true

	g.SetManagerFunc(e.layout)

	if err := e.keybindings(g); err != nil {
		return nil, err
	}

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		return nil, err
	}
	patches := make([]string, 0)
	for _, h := range e.State.editorHunks {
		if h.staged {
			patches = append(patches, generatePatch(e.State.File, h))
		}
	}
	return patches, nil
}

func (e *Editor) quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func (e *Editor) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	if v, err := g.SetView("main", -1, -1, maxX-1, maxY-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		v.Title = "Editor"
		v.Wrap = false
		e.updateView(0)
		v.SetCursor(0, headerLength)
	}
	if v, err := g.SetView("prompt", -1, maxY-2, maxX-1, maxY); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		v.Wrap = false
		fmt.Fprintf(v, ":")
	}
	g.SetCurrentView("main")
	e.updateView(0)
	return nil
}

func (e *Editor) updateView(index int) error {
	view, err := e.g.View("main")
	if err != nil {
		return err
	}
	view.Clear()
	out := bold.Sprint(hiWhite.Sprint(e.State.File.DiffHeader))
	fmt.Fprintln(view, out)
	for _, ehunk := range e.State.editorHunks {
		block := "█"
		if ehunk.selected {
			block = "▚"
		}
		if ehunk.staged {
			block = green.Sprint(block)
		} else {
			block = red.Sprint(block)
		}
		for _, ln := range hunkLines(ehunk.hunk) {
			fmt.Fprintf(view, "%s %s\n", block, ln)
		}
	}
	_, cy := view.Cursor()
	e.padMainView(cy)
	e.hitBottom()
	return nil
}

func tabsToWhitespace(input string) string {
	return strings.Replace(input, "\t", strings.Repeat(" ", 7), -1)
}

func hunkLines(hunk *diffparser.DiffHunk) []string {
	lines := make([]string, 0)
	lines = append(lines, cyan.Sprint(fmt.Sprintf("@@ -%d,%d +%d,%d @@ ", hunk.OrigRange.Start, hunk.OrigRange.Length, hunk.NewRange.Start, hunk.NewRange.Length))+
		tabsToWhitespace(hunk.HunkHeader))
	for _, line := range hunk.WholeRange.Lines {
		switch line.Mode {
		case diffparser.ADDED:
			lines = append(lines, green.Sprint(fmt.Sprintf("+%s", tabsToWhitespace(line.Content))))
		case diffparser.REMOVED:
			lines = append(lines, red.Sprint(fmt.Sprintf("-%s", tabsToWhitespace(line.Content))))
		default:
			lines = append(lines, fmt.Sprintf(" %s", tabsToWhitespace(line.Content)))
		}
	}
	return lines
}

func hunkString(hunk *diffparser.DiffHunk) string {
	out := fmt.Sprintf("@@ -%d,%d +%d,%d @@ ", hunk.OrigRange.Start, hunk.OrigRange.Length, hunk.NewRange.Start, hunk.NewRange.Length) +
		hunk.HunkHeader
	for _, line := range hunk.WholeRange.Lines {
		switch line.Mode {
		case diffparser.ADDED:
			out = out + "\n" + "+"
		case diffparser.REMOVED:
			out = out + "\n" + "-"
		default:
			out = out + "\n" + " "
		}
		out = out + line.Content
	}
	return out
}

func (e *Editor) stageHunk(g *gocui.Gui, v *gocui.View) error {
	hunks := e.State.editorHunks
	for _, hunk := range hunks {
		if hunk.selected {
			hunk.staged = !hunk.staged
		}
	}
	e.updateView(0)
	return nil
}

func generatePatch(file *diffparser.DiffFile, hunk *editorHunk) string {
	patch := file.DiffHeader
	patch = patch + "\n" + hunkString(hunk.hunk)
	return patch
}

func (e *Editor) setHunk(line int) error {
	currentTotal := 0
	for _, h := range e.State.editorHunks {
		currentTotal += h.hunk.Length()
		if currentTotal > line-headerLength {
			e.setActiveHunk(h)
			break
		}
	}
	return nil
}

func (e *Editor) setActiveHunk(hunk *editorHunk) {
	for _, h := range e.State.editorHunks {
		if h.selected {
			h.selected = false
		}
	}
	hunk.selected = true
}

func (e *Editor) cursorDown(g *gocui.Gui, v *gocui.View) error {
	if v != nil {
		cx, cy := v.Cursor()
		ox, oy := v.Origin()
		// magic number? (header and ?)
		if cy+oy > e.totalDiffLines()-2 {

		} else {
			if err := v.SetCursor(cx, cy+1); err != nil {

				if err := v.SetOrigin(ox, oy+1); err != nil {
					return err
				}
			}
		}
	}
	_, ucy := v.Cursor()
	_, uoy := v.Origin()
	e.setHunk(ucy + uoy)
	e.updateView(0)
	return nil
}

func (e *Editor) cursorUp(g *gocui.Gui, v *gocui.View) error {
	if v != nil {
		ox, oy := v.Origin()
		cx, cy := v.Cursor()
		if err := v.SetCursor(cx, cy-1); err != nil && oy > 0 {
			if err := v.SetOrigin(ox, oy-1); err != nil {
				return err
			}
		}
	}
	_, ucy := v.Cursor()
	_, uoy := v.Origin()
	e.setHunk(ucy + uoy)
	e.updateView(0)
	return nil
}
func (e *Editor) nextHunk(g *gocui.Gui, v *gocui.View) error {
	currentTotal := headerLength
	for _, h := range e.State.editorHunks {
		currentTotal += h.hunk.Length()
		if h.selected {
			break
		}
	}
	_, sy := v.Size()
	total := e.totalDiffLines()
	var anchor int
	var newcy int
	if currentTotal < sy {
		newcy = currentTotal
		anchor = 0
	} else if currentTotal < total {
		anchor = currentTotal
		newcy = 0
	} else {
		return nil
	}
	cx, _ := v.Cursor()
	if newcy > total-1 {
		return nil
	}
	if err := v.SetCursor(cx, newcy); err == nil {
		ox, _ := v.Origin()
		if err := v.SetOrigin(ox, anchor); err != nil {
			return err
		}
	}
	_, ucy := v.Cursor()
	_, uoy := v.Origin()
	e.setHunk(ucy + uoy)
	e.updateView(0)
	return nil
}

func (e *Editor) prevHunk(g *gocui.Gui, v *gocui.View) error {
	_, ucy := v.Cursor()
	_, uoy := v.Origin()
	var newcy, anchor int
	currentTotal := headerLength
	for idx, h := range e.State.editorHunks {
		currentTotal += h.hunk.Length()
		if h.selected {
			if idx == 0 {
				newcy = headerLength
				anchor = 0
				break
			}
			currentTotal = currentTotal - h.hunk.Length() - e.State.editorHunks[idx-1].hunk.Length()
			anchor = currentTotal
			break
		}
	}
	cx, _ := v.Cursor()
	if err := v.SetCursor(cx, newcy); err == nil {
		ox, _ := v.Origin()
		if err := v.SetOrigin(ox, anchor); err != nil {
			return err
		}
	}
	_, ucy = v.Cursor()
	_, uoy = v.Origin()
	e.setHunk(ucy + uoy)
	e.updateView(0)
	return nil
}

func (e *Editor) goTop(g *gocui.Gui, v *gocui.View) error {
	cx, _ := v.Cursor()
	if err := v.SetCursor(cx, 0); err == nil {
		ox, _ := v.Origin()
		if err := v.SetOrigin(ox, 0); err != nil {
			return err
		}
	}
	_, ucy := v.Cursor()
	_, uoy := v.Origin()
	e.setHunk(ucy + uoy)
	e.updateView(0)
	return nil
}

func (e *Editor) goBottom(g *gocui.Gui, v *gocui.View) error {
	bot := e.totalDiffLines()
	_, sy := v.Size()
	cx, _ := v.Cursor()
	if bot < sy {
		if err := v.SetCursor(cx, bot-1); err == nil {
			ox, _ := v.Origin()
			if err := v.SetOrigin(ox, 0); err != nil {
				return err
			}
		}
	} else {
		if err := v.SetCursor(cx, sy-1); err == nil {
			ox, _ := v.Origin()
			if err := v.SetOrigin(ox, bot-sy); err != nil {
				return err
			}
		}
	}

	_, ucy := v.Cursor()
	_, uoy := v.Origin()
	e.setHunk(ucy + uoy)
	e.updateView(0)

	return nil
}

func (e *Editor) padMainView(cur int) error {
	view, err := e.g.View("main")
	if err != nil {
		return err
	}
	_, sy := view.Size()
	fmt.Fprintf(view, strings.Repeat(bold.Sprint("~\n"), sy-cur))
	return nil
}

func (e *Editor) totalDiffLines() int {
	totalLines := headerLength
	for _, eh := range e.State.editorHunks {
		totalLines += eh.hunk.Length()
	}
	return totalLines
}

func (e *Editor) hitBottom() bool {
	p, err := e.g.View("prompt")
	if err != nil {
		return false
	}
	p.Clear()
	fmt.Fprintf(p, ":")
	tdl := e.totalDiffLines()
	v, err := e.g.View("main")
	if err != nil {
		return false
	}
	_, sy := v.Size()
	_, oy := v.Origin()
	if oy+sy >= tdl {
		p.Clear()
		fmt.Fprintf(p, black.Sprint(whitebg.Sprint("(END)")))
		return true
	}
	return false
}
