//go:build !windows

package cleaner

import (
	"os"
	"syscall"
)

// isFileLocked checks if the file has an active exclusive lock held by another process
func isFileLocked(file *os.File) bool {
	err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
			return true
		}
	} else {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	}
	return false
}
