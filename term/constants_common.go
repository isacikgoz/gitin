package term

const (
	esc = "\033["
	// HideCursor writes the sequence for hiding cursor
	hideCursor = "\x1b[?25l"
	// ShowCursor writes the sequence for resotring show cursor
	showCursor = "\x1b[?25h"
	// LineWrapOff sets the terminal to avoid line wrap
	lwoff = "\x1b[?7l"
	// LineWrapOn restores the linewrap setting
	lwon = "\x1b[?7h"
)

var (
	clearLine = []byte(esc + "2K\r")
	moveUp    = []byte(esc + "1A")
	moveDown  = []byte(esc + "1B")
)
