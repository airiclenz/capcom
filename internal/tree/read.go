package tree

import (
	"io"
	"io/fs"
	"os"
)

// maxNodeFileSize is the largest file read through the guarded reader, ReadRegularFile, in bytes:
// 1 MiB. The walk leaves out a larger node file too. ErrTooLarge's text names it.
const maxNodeFileSize = 1 << 20

// nonRegularModes are the mode bits of an entry that is not a regular file. ModeIrregular is left
// out on purpose: Windows reports it for some ordinary files, which read like any other.
const nonRegularModes = fs.ModeSymlink | fs.ModeDir | fs.ModeNamedPipe | fs.ModeSocket |
	fs.ModeDevice | fs.ModeCharDevice

// readOp is the Op of the *fs.PathError ReadRegularFile refuses a file with.
const readOp = "read"

// beforeOpen, when set, is called with the path ReadRegularFile reads between its Lstat and its
// open, so a test can swap the entry there. It is nil in production.
var beforeOpen func(path string)

// ReadRegularFile is the one guarded reader for repository-controlled files: node files, and any
// other file capcom reads from the working tree. It refuses an entry that is not a regular file
// with ErrNotRegular, and one larger than 1 MiB with ErrTooLarge. It never follows a symlink and
// never blocks on a named pipe: it checks the entry by its Lstat, opens it without following a
// link, checks the opened file again and reads at most one byte past the limit, so an entry
// swapped or grown after the first check is refused too. Every error is an *fs.PathError naming
// path: the refusals wrap the sentinel, and a failed stat, open or read is the os package's own.
func ReadRegularFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if err := checkReadable(path, info); err != nil {
		return nil, err
	}
	if beforeOpen != nil {
		beforeOpen(path)
	}

	file, err := openNoFollow(path, info)
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

// ReadNodeFile reads the store file at path: a node file, or any other file capcom keeps under
// .capcom/. It returns exactly what ReadRegularFile returns, refusals included.
func ReadNodeFile(path string) ([]byte, error) {
	return ReadRegularFile(path)
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
