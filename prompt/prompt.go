package prompt

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/fatih/color"
	"github.com/isacikgoz/gitin/term"
)

type keyEvent struct {
	ch  rune
	err error
}

type keyHandlerFunc func(rune) error
type selectionHandlerFunc func() error
type itemRendererFunc func(interface{}, []int, bool) []term.Cell
type informationRendererFunc func(interface{}) [][]term.Cell

//OptionalFunc handles functional arguments of the prompt
type OptionalFunc func(*Prompt)

// Options is the common options for building a prompt
type Options struct {
	LineSize      int `default:"5"`
	StartInSearch bool
	DisableColor  bool
}

// State holds the changeable vars of the prompt
type State struct {
	List        *List
	SearchMode  bool
	SearchStr   string
	SearchLabel string
	Cursor      int
	Scroll      int
	ListSize    int
}

// Prompt is a interactive prompt for command-line
type Prompt struct {
	list *List
	opts *Options

	keyHandler          keyHandlerFunc
	selectionHandler    selectionHandlerFunc
	itemRenderer        itemRendererFunc
	informationRenderer informationRendererFunc

	exitMsg  [][]term.Cell     // to be set on runtime if required
	Controls map[string]string // to be updated if additional controls added

	inputMode  bool
	helpMode   bool
	itemsLabel string
	input      string

	reader *term.RuneReader     // initialized by prompt
	writer *term.BufferedWriter // initialized by prompt
	mx     *sync.RWMutex

	events    chan keyEvent
	interrupt chan struct{}
	quit      chan struct{}
	hold      bool
}

// Create returns a pointer to prompt that is ready to Run
func Create(label string, opts *Options, list *List, fs ...OptionalFunc) *Prompt {
	p := &Prompt{
		opts:       opts,
		list:       list,
		itemsLabel: label,
	}

	p.keyHandler = p.onKey
	p.selectionHandler = p.onSelect
	p.itemRenderer = itemText
	p.informationRenderer = p.genInfo

	var mx sync.RWMutex
	p.mx = &mx

	p.reader = term.NewRuneReader(os.Stdin)
	p.writer = term.NewBufferedWriter(os.Stdout)

	p.events = make(chan keyEvent, 20)
	p.interrupt = make(chan struct{})
	p.quit = make(chan struct{})

	for _, f := range fs {
		f(p)
	}
	return p
}

// WithKeyHandler adds a keyHandler to the prompt
func WithKeyHandler(f keyHandlerFunc) OptionalFunc {
	return func(p *Prompt) {
		p.keyHandler = f
	}
}

// WithSelectionHandler adds a selection handler to the prompt
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
func (p *Prompt) Run() error {
	// disable echo and hide cursor
	if err := term.Init(os.Stdin, os.Stdout); err != nil {
		return err
	}
	defer term.Close()
	if p.opts.DisableColor {
		term.DisableColor()
	}
	if p.opts.StartInSearch {
		p.inputMode = true
	}

	// start input loop
	go p.spawnEvents()

	if err := p.mainloop(); err != nil {
		return err
	}

	for _, cells := range p.exitMsg {
		p.writer.WriteCells(cells)
	}
	p.writer.Flush()

	return nil
}

// Stop sends a quit signal to the main loop of the prompt
func (p *Prompt) Stop() {
	p.interrupt <- struct{}{}
}

func (p *Prompt) spawnEvents() {
	for {
		select {
		case <-p.interrupt:
			p.quit <- struct{}{}
			close(p.events)
			break
		default:
			time.Sleep(10 * time.Millisecond)
			if p.hold {
				continue
			}
			r, _, err := p.reader.ReadRune()
			p.events <- keyEvent{ch: r, err: err}
		}
	}
}

// this is the main loop for reading input channel
func (p *Prompt) mainloop() error {
	var err error
	sigwinch := make(chan os.Signal, 1)
	defer close(sigwinch)
	signal.Notify(sigwinch, syscall.SIGWINCH)
	p.render()

mainloop:
	for {
		select {
		case <-p.quit:
			break mainloop
		case ev := <-p.events:
			p.hold = true
			if err := ev.err; err != nil {
				return err
			}
			switch r := ev.ch; r {
			case rune(term.KeyCtrlC), rune(term.KeyCtrlD):
				break mainloop
			case term.Enter, term.NewLine:
				if err = p.selectionHandler(); err != nil {
					break mainloop
				}
			default:
				if err = p.keyBindings(r); err != nil {
					break mainloop
				}
			}
			p.render()
			p.hold = false
		case <-sigwinch:
			p.render()
		}
	}
	// reset cursor position and remove buffer
	p.writer.Reset()
	p.writer.ClearScreen()
	return err
}

// render function draws screen's list to terminal
func (p *Prompt) render() {
	// lock screen mutex
	p.mx.Lock()
	defer func() {
		p.writer.Flush()
		p.mx.Unlock()
	}()

	if p.helpMode {
		for _, line := range genHelp(p.allControls()) {
			p.writer.WriteCells(line)
		}
		return
	}

	items, idx := p.list.Items()
	p.writer.WriteCells(renderSearch(p.itemsLabel, p.inputMode, p.input))

	for i := range items {
		output := p.itemRenderer(items[i], p.list.matches[items[i]], (i == idx))
		p.writer.WriteCells(output)
	}

	p.writer.WriteCells(nil) // add an empty line
	if idx != NotFound {
		for _, line := range p.informationRenderer(items[idx]) {
			p.writer.WriteCells(line)
		}
	} else {
		p.writer.WriteCells(term.Cprint("Not found.", color.FgRed))
	}
}

func (p *Prompt) keyBindings(key rune) error {
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
			// p.input = ""
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
				p.input += string(key)
			}
			p.list.Search(p.input)
		} else if key == '?' {
			p.helpMode = !p.helpMode
		} else if key == 'h' || key == 'j' || key == 'k' || key == 'l' {
			switch key {
			case 'k':
				p.list.Prev()
			case 'j':
				p.list.Next()
			case 'h':
				p.list.PageDown()
			case 'l':
				p.list.PageUp()
			}
		} else {
			return p.keyHandler(key)
		}
	}
	return nil
}

func (p *Prompt) allControls() map[string]string {
	controls := make(map[string]string)
	controls["navigation"] = "← ↓ ↑ → (h,j,k,l)"
	controls["quit app"] = "q"
	controls["toggle search"] = "/"
	for k, v := range p.Controls {
		controls[k] = v
	}
	return controls
}

// onKey is the default keybinding function for a prompt
func (p *Prompt) onKey(key rune) error {
	switch key {
	case 'q':
		p.Stop()
	default:
	}
	return nil
}

// onSelect is the default selection
func (p *Prompt) onSelect() error {
	items, idx := p.list.Items()
	if idx == NotFound {
		return fmt.Errorf("could not select an item")
	}
	p.writer.WriteCells(term.Cprint(fmt.Sprint(items[idx])))
	return nil
}

// genInfo is the default function to genereate info
func (p *Prompt) genInfo(item interface{}) [][]term.Cell {
	return nil
}

// State return the current replace-able vars as a struct
func (p *Prompt) State() *State {
	scroll := p.list.Start()
	return &State{
		List:        p.list,
		SearchMode:  p.inputMode,
		SearchStr:   p.input,
		SearchLabel: p.itemsLabel,
		Cursor:      p.list.cursor,
		Scroll:      scroll,
		ListSize:    p.list.size,
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

// Selection returns the selected item
func (p *Prompt) Selection() (interface{}, error) {
	items, idx := p.list.Items()
	if idx == NotFound {
		return nil, fmt.Errorf("there is no item to be selected")
	}
	return items[idx], nil
}

// SetExitMsg adds a rendered cell grid to be printed after prompt is finished
func (p *Prompt) SetExitMsg(grid [][]term.Cell) {
	p.exitMsg = grid
}
