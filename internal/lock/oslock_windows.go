package lock

import (
	"os"

	"golang.org/x/sys/windows"
)

// lockedBytes is the length of the byte range the guard locks; any one byte serves.
const lockedBytes = 1

// lockFile blocks until it holds an exclusive LockFileEx lock on file. The lock binds to the file
// handle, so two opens in one process exclude each other just as two processes do.
func lockFile(file *os.File) error {
	overlapped := new(windows.Overlapped)
	return windows.LockFileEx(
		windows.Handle(file.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK,
		0,
		lockedBytes,
		0,
		overlapped,
	)
}

// unlockFile releases the lock lockFile took.
func unlockFile(file *os.File) error {
	overlapped := new(windows.Overlapped)
	return windows.UnlockFileEx(windows.Handle(file.Fd()), 0, lockedBytes, 0, overlapped)
}
