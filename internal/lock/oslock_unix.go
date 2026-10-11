//go:build unix

package lock

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// lockFile blocks until it holds an exclusive flock on file. flock binds to the open file, so
// two opens in one process exclude each other just as two processes do.
func lockFile(file *os.File) error {
	for {
		err := unix.Flock(int(file.Fd()), unix.LOCK_EX)
		if !errors.Is(err, unix.EINTR) {
			return err
		}
	}
}

// unlockFile releases the flock lockFile took.
func unlockFile(file *os.File) error {
	return unix.Flock(int(file.Fd()), unix.LOCK_UN)
}
