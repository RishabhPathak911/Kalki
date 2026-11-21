//go:build windows

package wal

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var lockFileEx = syscall.NewLazyDLL("kernel32.dll").NewProc("LockFileEx")

func lock(f *os.File) error {
	var overlapped syscall.Overlapped
	r, _, _ := lockFileEx.Call(f.Fd(), 3, 0, 1, 0, uintptr(unsafe.Pointer(&overlapped)))
	if r == 0 {
		return fmt.Errorf("WAL is already in use or cannot be locked")
	}
	return nil
}
