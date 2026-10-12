package tree

import "testing"

// SetMoveRename replaces the rename a Move makes its renames with, so a test can inject a failure.
func SetMoveRename(move *Move, rename func(oldPath string, newPath string) error) {
	move.rename = rename
}

// SetRemovalRemove replaces the remove a Removal removes its directories and files with, cleanup
// included, so a test can inject a failure.
func SetRemovalRemove(removal *Removal, remove func(path string) error) {
	removal.remove = remove
}

// SetLink replaces the hard link WriteFileExclusive names its file with until t ends, so a test can
// force the copy fallback. It swaps a package variable: a test that calls it never runs in parallel.
func SetLink(t *testing.T, fn func(oldPath string, newPath string) error) {
	t.Helper()
	saved := link
	link = fn
	t.Cleanup(func() { link = saved })
}
