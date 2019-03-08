package prompt

import (
	"os"
	"sync"

	"github.com/isacikgoz/fig/git"
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

	list   *List
	reader *reader.RuneReader
	writer *writer.BufferedWriter
	mx     *sync.RWMutex
	opts   *Options
}

func (p *prompt) start(runfunc runner) error {
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

	// disable echo
	p.reader.SetTermMode()
	defer p.reader.RestoreTermMode()

	// disable linewrap
	p.reader.Terminal.Out.Write([]byte(writer.HideCursor))
	defer p.reader.Terminal.Out.Write([]byte(writer.ShowCursor))

	return runfunc()
}

// render function draws screen's list to terminal
func (p *prompt) render(repo *git.Repository) {
	// make terminal not line wrap
	p.reader.Terminal.Out.Write([]byte(writer.LineWrapOff))
	defer p.reader.Terminal.Out.Write([]byte(writer.LineWrapOn))

	// lock screen mutex
	p.mx.Lock()
	defer p.mx.Unlock()

	items, idx := p.list.Items()

	if p.layout == status {
		if len(items) <= 0 && repo.Head != nil {
			for _, line := range branchClean(repo.Head) {
				p.writer.Write([]byte(line))
			}
			p.writer.Flush()
			return
		}
	}

	p.writer.Write([]byte(faint.Sprint(p.opts.SearchLabel)))

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
		for _, line := range branchInfo(repo.Head) {
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
