//go:build windows

package cleaner

import (
	"os"

	"golang.org/x/sys/windows"
)

// isFileLocked checks if the file has an active exclusive lock held by another process on Windows
func isFileLocked(file *os.File) bool {
	var overlapped windows.Overlapped
	handle := windows.Handle(file.Fd())

	// Try to acquire exclusive lock immediately
	err := windows.LockFileEx(handle, windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &overlapped)
	if err != nil {
		// If lock failed, another process holds it
		return true
	}

	// Unlock immediately since we only wanted to test the lock
	_ = windows.UnlockFileEx(handle, 0, 1, 0, &overlapped)
	return false
}
