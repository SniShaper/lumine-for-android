//go:build !linux && !android

package mobile

import "errors"

func dupFd(fd int) (int, error) {
	return 0, errors.New("fd duplication is only supported on unix-like targets")
}

func closeFd(fd int) error {
	return errors.New("fd duplication is only supported on unix-like targets")
}
