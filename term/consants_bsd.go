// +build darwin dragonfly freebsd netbsd openbsd

package term

import "syscall"

const (
	ioctlReadTermios  = syscall.TIOCGETA
	ioctlWriteTermios = syscall.TIOCSETA
)
