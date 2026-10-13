package store

import "testing"

// makeFIFO skips the test: Windows has no named pipes in the file system.
func makeFIFO(t *testing.T, _ string) {
	t.Helper()
	t.Skip("Windows has no named pipes in the file system")
}
