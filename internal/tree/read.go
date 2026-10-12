package tree

import (
	"io"
	"io/fs"
	"os"
)

// maxNodeFileSize is the largest store file read, in bytes: 1 MiB. ErrTooLarge's text names it.
const maxNodeFileSize = 1 << 20

// nonRegularModes are the mode bits of an entry that is not a regular file. ModeIrregular is left
// out on purpose: Windows reports it for some ordinary files, which read like any other.
const nonRegularModes = fs.ModeSymlink | fs.ModeDir | fs.ModeNamedPipe | fs.ModeSocket |
	fs.ModeDevice | fs.ModeCharDevice

// readOp is the Op of the *fs.PathError ReadNodeFile refuses a file with.
const readOp = "read"

// ReadNodeFile reads the store file at path: a node file, or any other file capcom keeps under
// .capcom/. It refuses, without opening it, an entry that is not a regular file (a symlink is not
// followed) with ErrNotRegular, and one larger than 1 MiB with ErrTooLarge; it checks both again on
// the opened file, and reads at most one byte past the limit, so a file swapped or grown after the
// first check is refused too. Every error is an *fs.PathError naming path: the refusals wrap the
// sentinel, and a failed stat, open or read is the os package's own.
func ReadNodeFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if err := checkReadable(path, info); err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if err := checkReadable(path, opened); err != nil {
		return nil, err
	}

	data, err := io.ReadAll(io.LimitReader(file, maxNodeFileSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxNodeFileSize {
		return nil, &fs.PathError{Op: readOp, Path: path, Err: ErrTooLarge}
	}
	return data, nil
}

// checkReadable refuses the file at path, as info describes it, when it is not a regular file or is
// larger than maxNodeFileSize.
func checkReadable(path string, info fs.FileInfo) error {
	if isNotRegular(info.Mode()) {
		return &fs.PathError{Op: readOp, Path: path, Err: ErrNotRegular}
	}
	if info.Size() > maxNodeFileSize {
		return &fs.PathError{Op: readOp, Path: path, Err: ErrTooLarge}
	}
	return nil
}

// isNotRegular reports whether mode is that of an entry other than a regular file.
func isNotRegular(mode fs.FileMode) bool {
	return mode&nonRegularModes != 0
}
