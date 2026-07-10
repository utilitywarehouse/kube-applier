//go:build linux || darwin

package envtestassets

import (
	"os"
	"syscall"
)

func lockFile(f *os.File) error {
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
		if err == nil {
			return nil
		}
		if err == syscall.EINTR {
			continue
		}
		return err
	}
}

func unlockFile(f *os.File) error {
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		if err == nil {
			return nil
		}
		if err == syscall.EINTR {
			continue
		}
		return err
	}
}
