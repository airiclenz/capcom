package nodefile

import "testing"

// SetBeforeRenameRetry sets the hook the replacing rename calls before each retry until t ends, so
// a test can change the target there. It swaps a package variable: a test that calls it never runs
// in parallel.
func SetBeforeRenameRetry(t *testing.T, fn func(path string)) {
	t.Helper()
	saved := beforeRenameRetry
	beforeRenameRetry = fn
	t.Cleanup(func() { beforeRenameRetry = saved })
}
