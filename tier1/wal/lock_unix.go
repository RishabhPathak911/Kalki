//go:build linux || darwin

package wal

import (
	"fmt"
	"os"
	"syscall"
)

func lock(f *os.File) error {
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fmt.Errorf("WAL is already in use")
	}
	return nil
}
