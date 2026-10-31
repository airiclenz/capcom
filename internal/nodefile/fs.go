package nodefile

import (
	"io"
	"io/fs"
	"os"
)

// FS is the file system the node tree is read and written through. Disk is the real one; a caller
// that must see writes before they reach the disk passes its own. Every method fails as its Disk
// counterpart does, each error naming the path it was given.
type FS interface {
	// Stat returns the FileInfo of the entry at path, following a symlink.
	Stat(path string) (fs.FileInfo, error)
	// Lstat returns the FileInfo of the entry at path, never following a symlink.
	Lstat(path string) (fs.FileInfo, error)
	// ReadDir lists the directory dir, its entries sorted by name.
	ReadDir(dir string) ([]fs.DirEntry, error)
	// ReadRegularFile reads the file at path through the guarded reader's rules: an entry that is
	// not a regular file is refused with ErrNotRegular and one over 1 MiB with ErrTooLarge.
	ReadRegularFile(path string) ([]byte, error)
	// ReplaceFile replaces the existing file at path with everything read from content, as
	// ReplaceFileAtomic does, failing with an error wrapping fs.ErrNotExist when path is missing.
	ReplaceFile(path string, content io.Reader) error
	// CreateFile creates the file at path with everything read from content, as
	// WriteFileExclusive does, failing with an error wrapping fs.ErrExist when path exists.
	CreateFile(path string, content io.Reader) error
}

// Disk is the FS of the real file system: os.Stat, os.Lstat and os.ReadDir, the guarded reader
// ReadRegularFile, and the crash-safe writes ReplaceFileAtomic and WriteFileExclusive.
type Disk struct{}

// Stat returns os.Stat of path.
func (Disk) Stat(path string) (fs.FileInfo, error) {
	return os.Stat(path)
}

// Lstat returns os.Lstat of path.
func (Disk) Lstat(path string) (fs.FileInfo, error) {
	return os.Lstat(path)
}

// ReadDir returns os.ReadDir of dir.
func (Disk) ReadDir(dir string) ([]fs.DirEntry, error) {
	return os.ReadDir(dir)
}

// ReadRegularFile reads the file at path with the guarded reader, ReadRegularFile.
func (Disk) ReadRegularFile(path string) ([]byte, error) {
	return ReadRegularFile(path)
}

// ReplaceFile replaces the existing file at path with ReplaceFileAtomic.
func (Disk) ReplaceFile(path string, content io.Reader) error {
	return ReplaceFileAtomic(path, content)
}

// CreateFile creates the file at path with WriteFileExclusive.
func (Disk) CreateFile(path string, content io.Reader) error {
	return WriteFileExclusive(path, content)
}
