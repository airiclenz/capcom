//go:build !unix

package nodefile

import (
	"io/fs"
	"os"
)

// openNoFollow opens the file at path for reading. With no way to refuse a symlink at the open,
// it refuses, with ErrNotRegular in an *fs.PathError naming path, a file that is not the entry
// lstatInfo describes: an entry swapped for a link after the caller's Lstat is read through by the
// open, but the file it reaches is not the one checked.
func openNoFollow(path string, lstatInfo fs.FileInfo) (*os.File, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	if !os.SameFile(lstatInfo, opened) {
		file.Close()
		return nil, &fs.PathError{Op: readOp, Path: path, Err: ErrNotRegular}
	}
	return file, nil
}
