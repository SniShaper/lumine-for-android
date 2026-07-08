//go:build windows

package platform

import "syscall"

type FD = syscall.Handle
