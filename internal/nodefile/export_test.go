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
// can force the copy fallback. It swaps a package variable: a test that calls it never runs in
// parallel.
func SetLink(t *testing.T, fn func(oldPath string, newPath string) error) {
	t.Helper()
	saved := link
	link = fn
	t.Cleanup(func() { link = saved })
}
