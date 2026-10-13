package nodefile

import "testing"

// SetBeforeOpen sets the hook ReadRegularFile calls between its Lstat and its open until t ends,
// so a test can swap the entry there. It swaps a package variable: a test that calls it never runs
// in parallel.
func SetBeforeOpen(t *testing.T, fn func(path string)) {
	t.Helper()
	saved := beforeOpen
	beforeOpen = fn
	t.Cleanup(func() { beforeOpen = saved })
}

// SetLink replaces the hard link WriteFileExclusive names its file with until t ends, so a test
// can force the no-replace rename. It swaps a package variable: a test that calls it never runs in
// parallel.
func SetLink(t *testing.T, fn func(oldPath string, newPath string) error) {
	t.Helper()
	saved := link
	link = fn
	t.Cleanup(func() { link = saved })
}

// SetNoReplaceRename replaces the no-replace rename WriteFileExclusive falls back to when the hard
// link is refused, until t ends, so a test can count its calls or force the in-place copy. It
// receives the platform's rename, so a counting fn can still call through to it. It swaps a
// package variable: a test that calls it never runs in parallel.
func SetNoReplaceRename(
	t *testing.T,
	fn func(platformRename func(oldPath string, newPath string) error, oldPath string, newPath string) error,
) {
	t.Helper()
	saved := renameNoReplace
	renameNoReplace = func(oldPath string, newPath string) error {
		return fn(saved, oldPath, newPath)
	}
	t.Cleanup(func() { renameNoReplace = saved })
}
