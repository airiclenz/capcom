package tree

import (
	"io/fs"
	"testing"
)

// SetMoveRename replaces the rename a Move makes its renames with, so a test can inject a failure.
func SetMoveRename(move *Move, rename func(oldPath string, newPath string) error) {
	move.rename = rename
}

// SetRemovalRemove replaces the remove a Removal removes its directories and files with, cleanup
// included, so a test can inject a failure.
func SetRemovalRemove(removal *Removal, remove func(path string) error) {
	removal.remove = remove
}

// SetLink replaces the hard link WriteFileExclusive names its file with, and a Move's node-file
// rename gives the file its new name with, until t ends, so a test can force the copy or rename
// fallback or plant a file at the target. It swaps a package variable: a test that calls it never
// runs in parallel.
func SetLink(t *testing.T, fn func(oldPath string, newPath string) error) {
	t.Helper()
	saved := link
	link = fn
	t.Cleanup(func() { link = saved })
}

// SetRemoveFile replaces the remove a Move's node-file rename removes the old name with, and takes
// a failed rename's new name back with, until t ends, so a test can inject a failure. It swaps a
// package variable: a test that calls it never runs in parallel.
func SetRemoveFile(t *testing.T, fn func(path string) error) {
	t.Helper()
	saved := removeFile
	removeFile = fn
	t.Cleanup(func() { removeFile = saved })
}

// SetBeforeOpen sets the hook ReadRegularFile calls between its Lstat and its open until t ends,
// so a test can swap the entry there. It swaps a package variable: a test that calls it never runs
// in parallel.
func SetBeforeOpen(t *testing.T, fn func(path string)) {
	t.Helper()
	saved := beforeOpen
	beforeOpen = fn
	t.Cleanup(func() { beforeOpen = saved })
}

// SetReadDir replaces the directory listing Walk reads every directory with until t ends, so a test
// can change the tree mid-walk. It swaps a package variable: a test that calls it never runs in
// parallel.
func SetReadDir(t *testing.T, fn func(dir string) ([]fs.DirEntry, error)) {
	t.Helper()
	saved := readDir
	readDir = fn
	t.Cleanup(func() { readDir = saved })
}
