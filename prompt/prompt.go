package prompt

import (
	"fmt"
	"os"
	"sync"
	"unicode/utf8"

	"github.com/fatih/color"
	"github.com/isacikgoz/gitin/term"
	git "github.com/isacikgoz/libgit2-api"
	"github.com/isacikgoz/sig/keys"
	"github.com/isacikgoz/sig/term"
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
type infobox func(Item) []string
type helpbox func() []string

// Options is the common options for building a prompt
type Options struct {
	Cursor           int
	Scroll           int
	Size             int
	ShowDetail       bool
	StartInSearch    bool
	SearchLabel      string
	InitSearchString string
	Finder           string
}

type prompt struct {
	layout promptType

	repo      *git.Repository
	list      *List
	keys      onKey
	selection onSelect
	info      infobox
	help      helpbox
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
	t := term.Terminal{
		In:  os.Stdin,
		Out: os.Stdout,
	}
	p.reader = term.NewRuneReader(t)
	p.writer = term.NewBufferedWriter(t.Out)
	p.list.SetCursor(p.opts.Cursor)
	p.list.SetStart(p.opts.Scroll)

	// disable echo
	p.reader.SetTermMode()
	defer p.reader.RestoreTermMode()

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
					for _, line := range branchClean(p.repo.Head) {
						p.writer.Write([]byte(line))
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
		if r == term.Interrupt {
			break
		}
		if r == term.EndTransmission {
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
		for _, line := range p.printHelp() {
			p.writer.Write([]byte(line))
		}
		p.writer.Flush()
		return
	}

	items, idx := p.list.Items()
	if p.inputMode {
		p.writer.Write([]byte(faint.Sprint("Search "+p.opts.SearchLabel) + " " + p.input + faint.Add(color.BlinkRapid).Sprint("█")))
	} else {
		p.writer.Write([]byte(faint.Sprint(p.opts.SearchLabel)))
	}

	// print each entry in the list
	var output []byte
	for i := range items {
		if i == idx {
			output = []byte(cyan.Sprint(">") + renderLine(items[i], nil))
		} else {
			output = []byte(" " + renderLine(items[i], nil))
		}
		p.writer.Write(output)
	}

	p.writer.Write([]byte(""))
	if idx != NotFound {
		for _, line := range p.info(items[idx]) {
			p.writer.Write([]byte(line))
		}
	}

	// finally, discharge to terminal
	p.writer.Flush()
}

func (p *prompt) assignKey(key rune) bool {
	var skipLoop bool
	if p.helpMode {
		p.helpMode = false
		return false
	}
	switch key {
	case term.Enter, '\n':
		return p.selection()
	case term.ArrowUp:
		skipLoop = true
		p.list.Prev()
	case term.ArrowDown:
		skipLoop = true
		p.list.Next()
	case term.ArrowLeft:
		skipLoop = true
		p.list.PageDown()
	case term.ArrowRight:
		skipLoop = true
		p.list.PageUp()
	default:
	}

	if skipLoop {

	} else if key == '/' {
		p.inputMode = !p.inputMode
		p.input = ""
	} else if p.inputMode {
		switch key {
		case rune(term.KeyBS), rune(term.KeyBS2):
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
	} else {
		return p.keys(key)
	}

	return false
}

func (p *prompt) printHelp() []string {
	var str []string
	str = append(str, fmt.Sprintf("%s: %s", faint.Sprint("navigation"), yellow.Sprint("← ↓ ↑ → (h,j,k,l)")))
	str = append(str, fmt.Sprintf("%s: %s", faint.Sprint("quit app"), yellow.Sprint("q")))
	str = append(str, fmt.Sprintf("%s: %s", faint.Sprint("toggle search"), yellow.Sprint("/")))
	if p.help != nil {
		str = append(str, p.help()...)
	}
	str = append(str, "")
	str = append(str, faint.Sprint("press any key to return."))
	return str
}
