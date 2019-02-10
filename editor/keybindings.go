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

// generate the editor controls a.k.a. keybindings
func (e *Editor) generateKeybindings() error {
	e.KeyBindings = make([]*KeyBinding, 0)
	mainKeys := []*KeyBinding{
		{
			View:        "",
			Key:         'q',
			Modifier:    gocui.ModNone,
			Handler:     e.quit,
			Display:     "q",
			Description: "Quit",
			Vital:       false,
		},
		{
			View:        "main",
			Key:         gocui.KeyArrowUp,
			Modifier:    gocui.ModNone,
			Handler:     e.cursorUp,
			Display:     "up",
			Description: "Cursor Up",
			Vital:       false,
		},
		{
			View:        "main",
			Key:         gocui.KeyArrowDown,
			Modifier:    gocui.ModNone,
			Handler:     e.cursorDown,
			Display:     "down",
			Description: "Cursor Down",
			Vital:       false,
		},
		{
			View:        "main",
			Key:         'k',
			Modifier:    gocui.ModNone,
			Handler:     e.cursorUp,
			Display:     "k",
			Description: "Cursor Up",
			Vital:       false,
		},
		{
			View:        "main",
			Key:         'j',
			Modifier:    gocui.ModNone,
			Handler:     e.cursorDown,
			Display:     "j",
			Description: "Cursor Down",
			Vital:       false,
		},
		{
			View:        "main",
			Key:         gocui.KeySpace,
			Modifier:    gocui.ModNone,
			Handler:     e.stageHunk,
			Display:     "space",
			Description: "Stage/Unstage",
			Vital:       false,
		},
		{
			View:        "main",
			Key:         'n',
			Modifier:    gocui.ModNone,
			Handler:     e.nextHunk,
			Display:     "n",
			Description: "Next hunk",
			Vital:       false,
		},
		{
			View:        "main",
			Key:         'N',
			Modifier:    gocui.ModNone,
			Handler:     e.prevHunk,
			Display:     "N",
			Description: "Previous hunk",
			Vital:       false,
		},
		{
			View:        "main",
			Key:         'g',
			Modifier:    gocui.ModNone,
			Handler:     e.goTop,
			Display:     "g",
			Description: "Go to top",
			Vital:       false,
		},
		{
			View:        "main",
			Key:         'G',
			Modifier:    gocui.ModNone,
			Handler:     e.goBottom,
			Display:     "G",
			Description: "Go to bottom",
			Vital:       false,
		},
	}
	e.KeyBindings = append(e.KeyBindings, mainKeys...)
	return nil
}

// set the guis by iterating over a slice of the gui's keybindings struct
func (e *Editor) keybindings(g *gocui.Gui) error {
	for _, k := range e.KeyBindings {
		if err := g.SetKeybinding(k.View, k.Key, k.Modifier, k.Handler); err != nil {
			return err
		}
	}
	return nil
}
