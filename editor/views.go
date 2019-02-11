package editor

import (
	"fmt"

	"github.com/jroimartin/gocui"
)

type View struct {
	name   string
	header string
}

var (
	main     = &View{name: "main", header: "Editor"}
	prompt   = &View{name: "prompt", header: ""}
	controls = &View{name: "controls", header: "Controls"}
	views    = []*View{main, controls}
)

func (e *Editor) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	if v, err := g.SetView(main.name, -1, -1, maxX-1, maxY-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		v.Wrap = false
		if _, err := g.SetCurrentView(main.name); err != nil {
			return err
		}
		e.updateView(0)
		v.SetCursor(0, headerLength)
	}
	if v, err := g.SetView(prompt.name, -1, maxY-2, maxX-1, maxY); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		v.Wrap = false
		fmt.Fprintf(v, ":")
	}
	kbw := int(0.50*float32(e.keyBindingWidth())) + 1
	kbh := int(0.50*float32(len(e.KeyBindings))) + 1 + len(views)

	hmX := int(0.50 * float32(maxX))
	hmY := int(0.50 * float32(maxY))
	if kbh > hmY {
		kbh = hmY - 1
	}
	if v, err := g.SetView(controls.name, hmX-kbw, hmY-kbh, hmX+kbw, hmY+kbh); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = true
		v.Title = controls.header
		v.Wrap = false
		g.SetViewOnBottom(controls.name)
	}
	e.updateView(0)
	return nil
}

func (v *View) String() string {
	return v.name
}
