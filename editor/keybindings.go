package editor

import "github.com/jroimartin/gocui"

// KeyBinding structs is helpful for not re-writinh the same function over and
// over again. it hold useful values to generate a controls view
type KeyBinding struct {
	View        string
	Handler     func(*gocui.Gui, *gocui.View) error
	Key         interface{}
	Modifier    gocui.Modifier
	Display     string
	Description string
	Vital       bool
}

type keyViewPair struct {
	key  interface{}
	view *View
}

// generate the editor controls a.k.a. keybindings
func (e *Editor) generateKeybindings() error {
	keymap := make(map[*keyViewPair]*KeyBinding)
	quit := &KeyBinding{
		Modifier:    gocui.ModNone,
		Handler:     e.quit,
		Display:     "q",
		Description: "Quit",
		Vital:       false,
	}
	keymap[&keyViewPair{'q', main}] = quit
	cursorUp := &KeyBinding{
		Modifier:    gocui.ModNone,
		Handler:     e.cursorUp,
		Display:     "↑, k",
		Description: "Cursor up",
		Vital:       false,
	}
	keymap[&keyViewPair{gocui.KeyArrowUp, main}] = cursorUp
	keymap[&keyViewPair{'k', main}] = cursorUp
	cursorDown := &KeyBinding{
		Modifier:    gocui.ModNone,
		Handler:     e.cursorDown,
		Display:     "↓, j",
		Description: "Cursor down",
		Vital:       false,
	}
	keymap[&keyViewPair{gocui.KeyArrowDown, main}] = cursorDown
	keymap[&keyViewPair{'j', main}] = cursorDown
	add := &KeyBinding{
		Modifier:    gocui.ModNone,
		Handler:     e.stageHunk,
		Display:     "space",
		Description: "Stage/Unstage",
		Vital:       false,
	}
	keymap[&keyViewPair{gocui.KeySpace, main}] = add
	nextHunk := &KeyBinding{
		Modifier:    gocui.ModNone,
		Handler:     e.nextHunk,
		Display:     "n",
		Description: "Next hunk",
		Vital:       false,
	}
	keymap[&keyViewPair{'n', main}] = nextHunk
	prevHunk := &KeyBinding{
		Modifier:    gocui.ModNone,
		Handler:     e.prevHunk,
		Display:     "N",
		Description: "Previous hunk",
		Vital:       false,
	}
	keymap[&keyViewPair{'N', main}] = prevHunk
	top := &KeyBinding{
		Modifier:    gocui.ModNone,
		Handler:     e.goTop,
		Display:     "g",
		Description: "Go to top",
		Vital:       false,
	}
	keymap[&keyViewPair{'g', main}] = top
	bottom := &KeyBinding{
		Modifier:    gocui.ModNone,
		Handler:     e.goBottom,
		Display:     "G",
		Description: "Go to bottom",
		Vital:       false,
	}
	keymap[&keyViewPair{'G', main}] = bottom
	openControls := &KeyBinding{
		Modifier:    gocui.ModNone,
		Handler:     e.createControlsView,
		Display:     "c",
		Description: "Open controls",
		Vital:       false,
	}
	keymap[&keyViewPair{'c', main}] = openControls
	quitControls := &KeyBinding{
		Modifier:    gocui.ModNone,
		Handler:     e.closeControlsView,
		Display:     "q",
		Description: "Close controls",
		Vital:       false,
	}
	keymap[&keyViewPair{'q', controls}] = quitControls
	e.KeyBindings = keymap
	return nil
}

// set the guis by iterating over a slice of the gui's keybindings struct
func (e *Editor) keybindings() error {
	for pair, bind := range e.KeyBindings {
		if err := e.g.SetKeybinding(pair.view.name, pair.key, bind.Modifier, bind.Handler); err != nil {
			return err
		}
	}
	return nil
}

func (e *Editor) keyBindingWidth() int {
	width := 10 // set minimum width
	for _, bind := range e.KeyBindings {
		if len(bind.Display)+len(bind.Description) > width {
			width = len(bind.Display) + len(bind.Description)
		}
	}
	return width + 4 // add some lines for clearance
}
