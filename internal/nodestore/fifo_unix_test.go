//go:build unix

package nodestore

import (
	"os"
	"syscall"
	"testing"
)

// makeFIFO creates a named pipe at path, skipping the test as root.
func makeFIFO(t *testing.T, path string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("named pipe tests do not run as root")
	}
	if err := syscall.Mkfifo(path, 0o644); err != nil {
		t.Fatalf("Mkfifo(%s): %v", path, err)
	}
}
