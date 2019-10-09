// +build !windows
package term

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

// Init initializes the term package
func Init(r Reader, w Writer) error {
	reader = r
	writer = w
	state = newTerminalState(reader)
	if _, _, err := unix.Syscall6(unix.SYS_IOCTL, uintptr(reader.Fd()), ioctlReadTermios, uintptr(unsafe.Pointer(&state.term)), 0, 0, 0); err != 0 {
		return err
	}

	newState := state.term
	// syscall.ECHO | syscall.ECHONL | syscall.ICANON to disable echo
	// syscall.ISIG is to catch keys like ctr-c or ctrl-d
	newState.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG

	if _, _, err := unix.Syscall6(unix.SYS_IOCTL, uintptr(reader.Fd()), ioctlWriteTermios, uintptr(unsafe.Pointer(&newState)), 0, 0, 0); err != 0 {
		return err
	}
	_, err := writer.Write([]byte(hideCursor))
	return err
}

// Close restores the terminal state
func Close() error {
	if _, _, err := unix.Syscall6(unix.SYS_IOCTL, uintptr(reader.Fd()), ioctlWriteTermios, uintptr(unsafe.Pointer(&state.term)), 0, 0, 0); err != 0 {
		return err
	}
	_, err := writer.Write([]byte(showCursor))
	return err
}
