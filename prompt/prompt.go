package prompt

import (
	"os"
	"sync"
	"unicode/utf8"

	"github.com/isacikgoz/fig/git"
	"github.com/isacikgoz/fig/search"

	"github.com/isacikgoz/sig/keys"
	"github.com/isacikgoz/sig/reader"
	"github.com/isacikgoz/sig/writer"
)

type promptType int

const (
	status promptType = iota
	log
	file
	branch
	stash
)

type runner func() error
type onKey func(rune) bool
type onSelect func() bool

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
	inputMode bool
	input     string
	reader    *reader.RuneReader
	writer    *writer.BufferedWriter
	mx        *sync.RWMutex
	opts      *Options
}

func (p *prompt) start() error {
	var mx sync.RWMutex
	p.mx = &mx
	term := reader.Terminal{
		In:  os.Stdin,
		Out: os.Stdout,
	}
	p.reader = reader.NewRuneReader(term)
	p.writer = writer.NewBufferedWriter(term.Out)
	p.list.SetCursor(p.opts.Cursor)
	p.list.SetStart(p.opts.Scroll)

	p.list.Searcher = search.FindFrom

	// disable echo
	p.reader.SetTermMode()
	defer p.reader.RestoreTermMode()

	// disable linewrap
	p.reader.Terminal.Out.Write([]byte(writer.HideCursor))
	defer p.reader.Terminal.Out.Write([]byte(writer.ShowCursor))

	return p.innerRun()
}

// this is the main loop for reading input
func (p *prompt) innerRun() error {
	var err error

	// start with first render
	p.render()

	// start waiting for input
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
				break
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
	// make terminal not line wrap
	p.reader.Terminal.Out.Write([]byte(writer.LineWrapOff))
	defer p.reader.Terminal.Out.Write([]byte(writer.LineWrapOn))

	// lock screen mutex
	p.mx.Lock()
	defer p.mx.Unlock()

	items, idx := p.list.Items()

	if p.layout == status && !p.inputMode {
		if len(items) <= 0 && p.repo.Head != nil {
			for _, line := range branchClean(p.repo.Head) {
				p.writer.Write([]byte(line))
			}
			p.writer.Flush()
			return
		}
	}

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

	if p.layout == status {
		// print repository status
		p.writer.Write([]byte(""))
		for _, line := range branchInfo(p.repo.Head) {
			p.writer.Write([]byte(line))
		}
	}

	// finally, discharge to terminal
	p.writer.Flush()
}

func (p *prompt) next() {
	p.list.Next()
}

func (p *prompt) previous() {
	p.list.Prev()
}

func (p *prompt) assignKey(key rune) bool {
	var skipLoop bool
	switch key {
	case keys.Enter, '\n':
		p.selection()
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
