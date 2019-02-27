// +build darwin dragonfly freebsd netbsd openbsd

package reader

import "syscall"

const (
	ioctlReadTermios  = syscall.TIOCGETA
	ioctlWriteTermios = syscall.TIOCSETA
)
