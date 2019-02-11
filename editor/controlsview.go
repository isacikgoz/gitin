package editor

import (
	"fmt"
	"strings"

	"github.com/jroimartin/gocui"
)

// render controls
func (e *Editor) createControlsView(g *gocui.Gui, v *gocui.View) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	v, err := g.View(controls.name)
	if err != nil {
		return err
	}
	if _, err := g.SetViewOnTop(controls.name); err != nil {
		return err
	}
	if _, err := g.SetCurrentView(controls.name); err != nil {
		return err
	}
	v.Clear()
	binds := e.generateControls()
	sx, _ := v.Size()
	for _, vw := range views {
		fmt.Fprintf(v, "%s view\n", bold.Sprint(vw.header))
		fmt.Fprintf(v, "%s\n", strings.Repeat("-", sx))
		for bind, view := range binds {
			if vw == view {
				fmt.Fprintf(v, "• %s: %s\n", yellow.Sprint(bind.Display), bind.Description)
			}
		}
		fmt.Fprintf(v, "\n")
	}
	g.Cursor = false
	return nil
}

// head back to diffhunk editor
func (e *Editor) closeControlsView(g *gocui.Gui, v *gocui.View) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	g.Cursor = true
	if _, err := g.SetViewOnBottom(controls.name); err != nil {
		return err
	}
	if _, err := g.SetViewOnTop(main.name); err != nil {
		return err
	}
	if _, err := g.SetCurrentView(main.name); err != nil {
		return err
	}
	return nil
}

// genreate controls-view map
func (e *Editor) generateControls() map[*KeyBinding]*View {
	controlmap := make(map[*KeyBinding]*View)
	for pair, bind := range e.KeyBindings {
		controlmap[bind] = pair.view
	}
	return controlmap
}
