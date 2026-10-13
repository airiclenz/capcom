package nodefile_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// makeSymlink creates link pointing at target, skipping the test on Windows and as root.
func makeSymlink(t *testing.T, target, link string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests do not run on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("symlink tests do not run as root")
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink(%s): %v", link, err)
	}
}

// writeSizedFile writes size bytes to path.
func writeSizedFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// writeNodes creates every relative path under dir as a file, making parent directories.
func writeNodes(t *testing.T, dir string, paths ...string) {
	t.Helper()
	for _, relative := range paths {
		path := filepath.Join(dir, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(path, []byte("x\n"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
}
