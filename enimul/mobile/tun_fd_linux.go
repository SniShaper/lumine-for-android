//go:build linux || android

package mobile

import "syscall"

func dupFd(fd int) (int, error) {
	return syscall.Dup(fd)
}

func closeFd(fd int) error {
	return syscall.Close(fd)
}
