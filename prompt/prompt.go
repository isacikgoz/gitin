package prompt

import (
	"os"
	"sync"
	"unicode/utf8"

	"github.com/fatih/color"
	"github.com/isacikgoz/gitin/term"
	git "github.com/isacikgoz/libgit2-api"
)

type promptType int

const (
	status promptType = iota
	log
	file
	branch
	stash
)

type onKey func(rune) bool
type onSelect func() bool
type infobox func(Item) [][]term.Cell

// Options is the common options for building a prompt
type Options struct {
	Cursor        int
	Scroll        int
	Size          int
	StartInSearch bool
	SearchLabel   string
}

type promptState struct {
	list       *List
	searchMode bool
	searchStr  string
	cursor     int
	scroll     int
}

type prompt struct {
	layout promptType

	repo      *git.Repository
	list      *List
	keys      onKey
	selection onSelect
	info      infobox
	controls  map[string]string
	inputMode bool
	helpMode  bool
	input     string
	reader    *term.RuneReader
	writer    *term.BufferedWriter
	mx        *sync.RWMutex
	opts      *Options
}

func (p *prompt) start() error {
	var mx sync.RWMutex
	p.mx = &mx

	p.reader = term.NewRuneReader(os.Stdin)
	p.writer = term.NewBufferedWriter(os.Stdout)
	p.list.SetCursor(p.opts.Cursor)
	p.list.SetStart(p.opts.Scroll)

	// disable echo and hide cursor
	if err := term.Init(os.Stdin, os.Stdout); err != nil {
		return err
	}
	defer term.Close()

	if p.opts.StartInSearch {
		p.inputMode = true
	}
	return p.innerRun()
}

// this is the main loop for reading input
func (p *prompt) innerRun() error {
	var err error

	// start with first render
	p.render()

	// start waiting for input
mainloop:
	for {
		switch p.layout {
		case status:
			items, _ := p.list.Items()
			if len(items) <= 0 && p.repo.Head != nil && !p.inputMode {
				defer func() {
					for _, line := range workingTreeClean(p.repo.Head) {
						p.writer.WriteCells(line)
					}
					p.writer.Flush()
				}()
				err = nil
				break mainloop
			}
		}
		r, _, err := p.reader.ReadRune()
		if err != nil {
			return err
		}
		if r == rune(term.KeyCtrlC) {
			break
		}
		if r == rune(term.KeyCtrlD) {
			break
		}
		if br := p.assignKey(r); br {
			break
		}
		p.render()
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
	defer p.mx.Unlock()

	if p.helpMode {
		for _, line := range genHelp(p.allControls()) {
			p.writer.WriteCells(line)
		}
		p.writer.Flush()
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

	// finally, discharge to terminal
	p.writer.Flush()
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
