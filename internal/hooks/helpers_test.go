package hooks_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/airiclenz/capcom/internal/nodefile"
)

// unusableFile is a way to put something other than a readable settings or .gitattributes file at
// a path, and the *hooks.Error message the installers refuse it with.
type unusableFile struct {
	name        string
	make        func(t *testing.T, path string)
	wantMessage string
}

// unusableFiles are the entries the installers must refuse without following or reading them: a
// symlink to a usable file, a named pipe and a file larger than 1 MiB.
var unusableFiles = []unusableFile{
	{"symlink", makeSymlinkToUsableFile, "not a regular file"},
	{"named pipe", makeFIFO, "not a regular file"},
	{"larger than 1 MiB", makeOversizedFile, "file is larger than 1 MiB"},
}

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

// makeSymlinkToUsableFile creates path as a symlink to an empty JSON object in a file beside it.
func makeSymlinkToUsableFile(t *testing.T, path string) {
	t.Helper()
	target := filepath.Join(filepath.Dir(path), "elsewhere")
	if err := os.WriteFile(target, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", target, err)
	}
	makeSymlink(t, target, path)
}

// makeOversizedFile writes one byte more than the guarded reader reads to path.
func makeOversizedFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, make([]byte, nodefile.MaxNodeFileSize+1), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

// entryMode returns the type bits and size of the entry at path, not following a symlink.
func entryMode(t *testing.T, path string) (os.FileMode, int64) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("Lstat(%s): %v", path, err)
	}
	return info.Mode().Type(), info.Size()
}
