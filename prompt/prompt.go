package prompt

import (
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

type onKey func(rune) bool
type onSelect func() bool
type genInfo func(Item) [][]term.Cell

// Options is the common options for building a prompt
type Options struct {
	Size          int
	StartInSearch bool
	SearchLabel   string
	DisableColor  bool
}

type promptState struct {
	list       *List
	searchMode bool
	searchStr  string
	cursor     int
	scroll     int
}

type prompt struct {
	list *List
	opts *Options

	keys      onKey
	selection onSelect
	info      genInfo

	exitMsg  [][]term.Cell     // to be set on runtime if required
	controls map[string]string // to be updated if additional controls added

	inputMode bool
	helpMode  bool
	input     string

	reader *term.RuneReader     // initialized by prompt
	writer *term.BufferedWriter // initialized by prompt
	mx     *sync.RWMutex

	events chan keyEvent
	quit   chan bool
	hold   bool
}

type optionalFunc func(*prompt)

func withOnKey(f onKey) optionalFunc {
	return func(p *prompt) {
		p.keys = f
	}
}

func withSelection(f onSelect) optionalFunc {
	return func(p *prompt) {
		p.selection = f
	}
}

func withInfo(f genInfo) optionalFunc {
	return func(p *prompt) {
		p.info = f
	}
}

func create(opts *Options, list *List, fs ...optionalFunc) *prompt {
	p := &prompt{
		opts: opts,
		list: list,
	}

	p.keys = p.onKey
	p.selection = p.onSelect
	p.info = p.genInfo

	var mx sync.RWMutex
	p.mx = &mx

	p.reader = term.NewRuneReader(os.Stdin)
	p.writer = term.NewBufferedWriter(os.Stdout)

	p.events = make(chan keyEvent, 20)
	p.quit = make(chan bool)

	for _, f := range fs {
		f(p)
	}
	return p
}

func (p *prompt) start() error {
	var mx sync.RWMutex
	p.mx = &mx

	p.reader = term.NewRuneReader(os.Stdin)
	p.writer = term.NewBufferedWriter(os.Stdout)

	p.events = make(chan keyEvent, 20)
	p.quit = make(chan bool)

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
	go p.spawnEvent()

	if err := p.innerRun(); err != nil {
		return err
	}

	for _, cells := range p.exitMsg {
		p.writer.WriteCells(cells)
	}
	p.writer.Flush()

	return nil
}

func (p *prompt) spawnEvent() {
	for {
		select {
		case <-p.quit:
			return
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
func (p *prompt) innerRun() error {
	var err error
	sigwinch := make(chan os.Signal, 1)
	signal.Notify(sigwinch, syscall.SIGWINCH)
	p.render()

mainloop:
	for {
		select {
		case ev := <-p.events:
			p.hold = true
			if err := ev.err; err != nil {
				return err
			}
			switch r := ev.ch; r {
			case rune(term.KeyCtrlC), rune(term.KeyCtrlD):
				break mainloop
			default:
				if br := p.assignKey(r); br {
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
func (p *prompt) render() {
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
	p.writer.WriteCells(renderSearch(p.opts.SearchLabel, p.inputMode, p.input))

	// print each entry in the list
	for i := range items {
		var output []term.Cell
		output = append(output, renderItem(items[i], p.list.matches[items[i]], (i == idx))...)
		p.writer.WriteCells(output)
	}

	p.writer.WriteCells(nil) // add an empty line
	if idx != NotFound {
		for _, line := range p.info(items[idx]) {
			p.writer.WriteCells(line)
		}
	} else {
		p.writer.WriteCells(term.Cprint("Not found.", color.FgRed))
	}
}

func (p *prompt) assignKey(key rune) bool {
	if p.helpMode {
		p.helpMode = false
		return false
	}
	switch key {
	case term.Enter, '\n':
		return p.selection()
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
			return p.keys(key)
		}
	}
	return false
}

func (p *prompt) allControls() map[string]string {
	controls := make(map[string]string)
	controls["navigation"] = "← ↓ ↑ → (h,j,k,l)"
	controls["quit app"] = "q"
	controls["toggle search"] = "/"
	for k, v := range p.controls {
		controls[k] = v
	}
	return controls
}

// onKey is the default keybinding function for a prompt
func (p *prompt) onKey(key rune) bool {
	switch key {
	case 'q':
		p.quit <- true
		return true
	default:
	}
	return false
}

// onSelect is the default selection
func (p *prompt) onSelect() bool {
	items, idx := p.list.Items()
	if idx == NotFound {
		return false
	}
	p.writer.WriteCells(term.Cprint(items[idx].String()))
	return false
}

// genInfo is the default function to genereate info
func (p *prompt) genInfo(item Item) [][]term.Cell {
	return nil
}
