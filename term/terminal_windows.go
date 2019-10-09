// +build windows
package term

import (
	"golang.org/x/sys/windows"
)

var (
	dll              = windows.NewLazyDLL("kernel32.dll")
	setConsoleMode   = dll.NewProc("SetConsoleMode")
	getConsoleMode   = dll.NewProc("GetConsoleMode")
	readConsoleInput = dll.NewProc("ReadConsoleInputW")
)

// Init initializes the term package
func Init(r Reader, w Writer) error {
	reader = r
	writer = w
	state = newTerminalState(reader)

	newState := state.term
	newState &^= ENABLE_ECHO_INPUT | ENABLE_LINE_INPUT | ENABLE_PROCESSED_INPUT
	r, _, err = setConsoleMode.Call(uintptr(reader.Fd()), uintptr(&newState))
	// windows return 0 on error
	if r == 0 {
		return err
	}
	return nil
}

// Close restores the terminal state
func Close() error {
	r, _, err := setConsoleMode.Call(uintptr(reader.Fd()), uintptr(&state.term))
	// windows return 0 on error
	if r == 0 {
		return err
	}
	return nil
}
