//go:build stress

package cli

import (
	"fmt"
	"os"
	"testing"
)

// TestMain runs the stress tests, then removes the directory stressBinary built the capcom
// binary into, so no capcom-stress-* directory outlives the run. A run that never built the
// binary leaves stressBuildDir empty and removes nothing. Untagged builds have no TestMain, so
// bench_test.go and the other tests of this package are unaffected.
func TestMain(m *testing.M) {
	code := m.Run()
	if stressBuildDir != "" {
		if err := os.RemoveAll(stressBuildDir); err != nil {
			fmt.Fprintf(os.Stderr, "remove stress build dir: %v\n", err)
		}
	}
	os.Exit(code)
}
