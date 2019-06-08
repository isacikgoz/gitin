package prompt

import (
	"os"
	"sync"
	"unicode/utf8"

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

// Options is the common options for building a prompt
type Options struct {
	Cursor           int
	Scroll           int
	Size             int
	HideHelp         bool
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
	inputMode bool
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
		if r == keys.Interrupt {
			break
		}
		if r == keys.EndTransmission {
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

	items, idx := p.list.Items()
	if p.inputMode {
		p.writer.Write([]byte(faint.Sprint("Search "+p.opts.SearchLabel) + " " + p.input))
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
	switch key {
	case keys.Enter, '\n':
		return p.selection()
	case keys.ArrowUp:
		skipLoop = true
		p.list.Prev()
	case keys.ArrowDown:
		skipLoop = true
		p.list.Next()
	case keys.ArrowLeft:
		skipLoop = true
		p.list.PageDown()
	case keys.ArrowRight:
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
		case rune(keys.KeyBS), rune(keys.KeyBS2):
			if len(p.input) > 0 {
				_, size := utf8.DecodeLastRuneInString(p.input)
				p.input = p.input[0 : len(p.input)-size]
			}
		case rune(keys.KeyCtrlU):
			p.input = ""
		default:
			p.input += string(key)
		}
		p.list.Search(p.input)
	} else {
		return p.keys(key)
	}

	return false
}
