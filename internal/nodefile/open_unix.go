//go:build unix

package nodefile

import (
	"errors"
	"io/fs"
	"os"

	"golang.org/x/sys/unix"
)

// openNoFollow opens the file at path for reading without following a symlink and without
// blocking on a named pipe, then puts the descriptor back into blocking mode. A symlink is refused
// with ErrNotRegular in an *fs.PathError naming path. It needs no identity check against the
// caller's Lstat, so it ignores that: O_NOFOLLOW rules out a followed link, and a named pipe or
// any other entry swapped in shows in the opened file's own mode, which the caller checks.
func openNoFollow(path string, _ fs.FileInfo) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if errors.Is(err, unix.ELOOP) {
		return nil, &fs.PathError{Op: readOp, Path: path, Err: ErrNotRegular}
	}
	if err != nil {
		return nil, err
	}
	if err := unix.SetNonblock(int(file.Fd()), false); err != nil {
		file.Close()
		return nil, &fs.PathError{Op: readOp, Path: path, Err: err}
	}
	return file, nil
}
