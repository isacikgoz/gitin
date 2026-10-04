package prompt

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/fatih/color"
	"github.com/isacikgoz/gitin/term"
)

type keyEvent struct {
	ch  rune
	err error
}

// KeyBinding is used for mapping a key to a function. An error returned by
// the handler is shown to the user, the prompt keeps running.
type KeyBinding struct {
	Key     rune
	Display string
	Handler func(interface{}) error
	Desc    string
}

type selectionHandlerFunc func(interface{}) error
type itemRendererFunc func(interface{}, []int, bool) [][]term.Cell
type informationRendererFunc func(interface{}) [][]term.Cell

// OptionalFunc handles functional arguments of the prompt
type OptionalFunc func(*Prompt)

// Options is the common options for building a prompt
type Options struct {
	LineSize      int `default:"5"`
	StartInSearch bool
	DisableColor  bool
	VimKeys       bool `default:"true"`
}

// State holds the changeable vars of the prompt
type State struct {
	List        List
	SearchMode  bool
	SearchStr   string
	SearchLabel string
	Cursor      int
	Scroll      int
	ListSize    int
}

// Prompt is a interactive prompt for command-line
type Prompt struct {
	list        List
	opts        *Options
	keyBindings []*KeyBinding

	selectionHandler    selectionHandlerFunc
	itemRenderer        itemRendererFunc
	informationRenderer informationRendererFunc

	exitMsg [][]term.Cell // to be set on runtime if required
	notice  string        // error of the last action, shown until the next key press

	inputMode  bool
	helpMode   bool
	itemsLabel string
	input      string

	reader *term.RuneReader     // initialized by prompt
	writer *term.BufferedWriter // initialized by prompt
	mx     *sync.RWMutex

	events chan keyEvent
	quit   chan error
}

// Create returns a pointer to prompt that is ready to Run
func Create(label string, opts *Options, list List, fs ...OptionalFunc) *Prompt {
	p := &Prompt{
		opts:         opts,
		list:         list,
		itemsLabel:   label,
		itemRenderer: itemText,
		reader:       term.NewRuneReader(os.Stdin),
		writer:       term.NewBufferedWriter(os.Stdout),
		mx:           &sync.RWMutex{},
		events:       make(chan keyEvent, 20),
		quit:         make(chan error, 1),
	}

	for _, f := range fs {
		f(p)
	}
	return p
}

// WithSelectionHandler adds a selection handler to the prompt. An error
// returned by the handler is shown to the user, the prompt keeps running.
func WithSelectionHandler(f selectionHandlerFunc) OptionalFunc {
	return func(p *Prompt) {
		p.selectionHandler = f
	}
}

// WithItemRenderer to add your own implementation on rendering an Item
func WithItemRenderer(f itemRendererFunc) OptionalFunc {
	return func(p *Prompt) {
		p.itemRenderer = f
	}
}

// WithInformation adds additional information below to the prompt
func WithInformation(f informationRendererFunc) OptionalFunc {
	return func(p *Prompt) {
		p.informationRenderer = f
	}
}

// Run as name implies starts the prompt until it quits
func (p *Prompt) Run(ctx context.Context) error {
	// disable echo and hide cursor
	if err := term.Init(os.Stdin, os.Stdout); err != nil {
		return err
	}
	defer func() { _ = term.Close() }()

	if p.opts.DisableColor {
		term.DisableColor()
	}

	if p.opts.StartInSearch {
		p.inputMode = true
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// start input loop
	go p.spawnEvents(ctx)

	p.render() // start with an initial render

	err := p.mainloop()

	// reset cursor position and remove buffer
	p.writer.Reset()
	_ = p.writer.ClearScreen()

	if err != nil {
		return err
	}

	for _, cells := range p.exitMsg {
		_, _ = p.writer.WriteCells(cells)
	}
	_ = p.writer.Flush()

	return nil
}

// Stop sends a quit signal to the main loop of the prompt
func (p *Prompt) Stop() {
	p.stop(nil)
}

// Fail stops the prompt and makes Run return err. It is safe to call from
// any goroutine.
func (p *Prompt) Fail(err error) {
	p.stop(err)
}

func (p *Prompt) stop(err error) {
	select {
	case p.quit <- err:
	default: // the prompt is already stopping
	}
}

func (p *Prompt) spawnEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Millisecond):
			p.mx.Lock()
			r, _, err := p.reader.ReadRune()
			p.mx.Unlock()
			p.events <- keyEvent{ch: r, err: err}
		}
	}
}

// this is the main loop for reading input channel
func (p *Prompt) mainloop() error {
	sigwinch := make(chan os.Signal, 1)
	signal.Notify(sigwinch, syscall.SIGWINCH)
	defer signal.Stop(sigwinch)

	for {
		select {
		case err := <-p.quit:
			return err
		case <-sigwinch:
			p.render()
		case <-p.list.Update():
			p.render()
		case ev := <-p.events:
			if err := func() error {
				p.mx.Lock()
				defer p.mx.Unlock()

				if err := ev.err; err != nil {
					return err
				}

				p.notice = ""
				var err error
				switch r := ev.ch; r {
				case rune(term.KeyCtrlC), rune(term.KeyCtrlD):
					p.Stop()
					return nil
				case term.Enter, term.NewLine:
					items, idx := p.list.Items()
					if idx == NotFound {
						break
					}
					err = p.selectionHandler(items[idx])
				default:
					err = p.onKey(r)
				}
				if err != nil {
					p.notice = err.Error()
				}
				p.render()
				return nil
			}(); err != nil {
				return err
			}
		}
	}
}

// render function draws screen's list to terminal
func (p *Prompt) render() {
	defer func() { _ = p.writer.Flush() }()

	if p.helpMode {
		for _, line := range genHelp(p.allControls()) {
			_, _ = p.writer.WriteCells(line)
		}
		return
	}

	items, idx := p.list.Items()
	_, _ = p.writer.WriteCells(renderSearch(p.itemsLabel, p.inputMode, p.input))

	for i := range items {
		output := p.itemRenderer(items[i], p.list.Matches(items[i]), (i == idx))
		for _, l := range output {
			_, _ = p.writer.WriteCells(l)
		}
	}

	_, _ = p.writer.WriteCells(nil) // add an empty line
	if idx != NotFound {
		for _, line := range p.informationRenderer(items[idx]) {
			_, _ = p.writer.WriteCells(line)
		}
	} else {
		_, _ = p.writer.WriteCells(term.Cprint("Not found.", color.FgRed))
	}

	for _, line := range noticeLines(p.notice) {
		_, _ = p.writer.WriteCells(term.Cprint(line, color.FgRed))
	}
}

// noticeLines returns the first few non-empty lines of an error message
func noticeLines(notice string) []string {
	const maxLines = 5
	var lines []string
	for _, line := range strings.Split(notice, "\n") {
		if line = strings.TrimSpace(line); line != "" && len(lines) < maxLines {
			lines = append(lines, line)
		}
	}
	return lines
}

// AddKeyBinding adds a key-function map to prompt
func (p *Prompt) AddKeyBinding(b *KeyBinding) error {
	p.keyBindings = append(p.keyBindings, b)
	return nil
}

// default key handling function
func (p *Prompt) onKey(key rune) error {
	if p.helpMode {
		p.helpMode = false
		return nil
	}

	switch key {
	case term.ArrowUp:
		p.list.Prev()
	case term.ArrowDown:
		p.list.Next()
	case term.ArrowLeft:
		p.list.PageDown()
	case term.ArrowRight:
		p.list.PageUp()
	default:

		if key == '/' {
			p.inputMode = !p.inputMode
		} else if p.inputMode {
			switch key {
			case term.Backspace, term.Backspace2:
				if len(p.input) > 0 {
					_, size := utf8.DecodeLastRuneInString(p.input)
					p.input = p.input[0 : len(p.input)-size]
				}
			case rune(term.KeyCtrlU):
				p.input = ""
			default:
				if !unicode.IsPrint(key) {
					return nil
				}
				p.input += string(key)
			}
			p.list.Search(p.input)
		} else if key == '?' {
			p.helpMode = !p.helpMode
		} else if p.opts.VimKeys && key == 'k' {
			// refactor vim keys
			p.list.Prev()
		} else if p.opts.VimKeys && key == 'j' {
			p.list.Next()
		} else if p.opts.VimKeys && key == 'h' {
			p.list.PageDown()
		} else if p.opts.VimKeys && key == 'l' {
			p.list.PageUp()
		} else {
			items, idx := p.list.Items()
			if idx == NotFound {
				return nil
			}

			for _, kb := range p.keyBindings {
				if kb.Key == key {
					return kb.Handler(items[idx])
				}
			}
		}
	}

	return nil
}

func (p *Prompt) allControls() map[string]string {
	controls := make(map[string]string)
	controls["← ↓ ↑ → (h,j,k,l)"] = "navigation"
	controls["/"] = "toggle search"
	for _, kb := range p.keyBindings {
		controls[kb.Display] = kb.Desc
	}
	return controls
}

// State return the current replace-able vars as a struct
func (p *Prompt) State() *State {
	scroll := p.list.Start()
	return &State{
		List:        p.list,
		SearchMode:  p.inputMode,
		SearchStr:   p.input,
		SearchLabel: p.itemsLabel,
		Cursor:      p.list.Cursor(),
		Scroll:      scroll,
		ListSize:    p.list.Size(),
	}
}

// SetState replaces the state of the prompt
func (p *Prompt) SetState(state *State) {
	p.list = state.List
	p.inputMode = state.SearchMode
	p.input = state.SearchStr
	p.itemsLabel = state.SearchLabel
	p.list.SetCursor(state.Cursor)
	p.list.SetStart(state.Scroll)
}

// ListSize returns the size of the items that is renderer each time
func (p *Prompt) ListSize() int {
	return p.opts.LineSize
}

// SetExitMsg adds a rendered cell grid to be printed after prompt is finished
func (p *Prompt) SetExitMsg(grid [][]term.Cell) {
	p.exitMsg = grid
}
