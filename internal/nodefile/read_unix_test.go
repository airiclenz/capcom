//go:build unix

package nodefile_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/airiclenz/capcom/internal/nodefile"
)

// swapTimeout bounds a read whose entry was swapped before the open, so a reader that blocks on
// a named pipe fails the test instead of hanging it.
const swapTimeout = 5 * time.Second

// readResult is what a guarded read returned.
type readResult struct {
	data []byte
	err  error
}

// TestReadNodeFile_SwappedBeforeOpen_RefusedNotRegular swaps the regular file the reader checked
// for another entry between its Lstat and its open. It swaps a package variable, so it never runs
// in parallel.
func TestReadNodeFile_SwappedBeforeOpen_RefusedNotRegular(t *testing.T) {
	cases := []struct {
		name string
		make func(t *testing.T, path string)
	}{
		{
			name: "symlink to another regular file",
			make: func(t *testing.T, path string) {
				target := filepath.Join(t.TempDir(), "target.md")
				writeSizedFile(t, target, 1)
				makeSymlink(t, target, path)
			},
		},
		{name: "named pipe", make: makeFIFO},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "node.md")
			writeSizedFile(t, path, 1)
			replacement := filepath.Join(t.TempDir(), "replacement")
			tc.make(t, replacement)
			var swapErr error
			nodefile.SetBeforeOpen(t, func(opened string) {
				if opened == path {
					swapErr = os.Rename(replacement, path)
				}
			})

			result := readWithTimeout(t, path)

			if swapErr != nil {
				t.Fatalf("swap: %v", swapErr)
			}
			assertRefused(t, "ReadNodeFile", path, result.data, result.err, nodefile.ErrNotRegular)
		})
	}
}

// readWithTimeout calls ReadNodeFile on path in a goroutine and fails t when it has not returned
// within swapTimeout.
func readWithTimeout(t *testing.T, path string) readResult {
	t.Helper()
	done := make(chan readResult, 1)
	go func() {
		data, err := nodefile.ReadNodeFile(path)
		done <- readResult{data: data, err: err}
	}()
	select {
	case result := <-done:
		return result
	case <-time.After(swapTimeout):
		t.Fatalf("ReadNodeFile(%s) did not return within %s", path, swapTimeout)
		return readResult{}
	}
}
