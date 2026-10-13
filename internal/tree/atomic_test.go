package tree_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/tree"
)

func TestWriteFileExclusive_FreePath_CreatesFileAsLFAndLeavesNoTempFile(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "login-flow")
	path := filepath.Join(dir, "login-timeout-race.md")

	err := tree.WriteFileExclusive(path, strings.NewReader("new\r\ncontent\n"))

	if err != nil {
		t.Fatalf("WriteFileExclusive: %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "new\ncontent\n" {
		t.Fatalf("content = %q", got)
	}
	if names := dirEntries(t, dir); !reflect.DeepEqual(names, []string{"login-timeout-race.md"}) {
		t.Fatalf("dir holds %v, want only login-timeout-race.md", names)
	}
}

func TestWriteFileExclusive_PathExists_FailsWithErrExistAndLeavesFileAndNoTempFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "login-flow.md")
	writeNodes(t, dir, "login-flow.md")

	err := tree.WriteFileExclusive(path, strings.NewReader("new\ncontent\n"))

	if !errors.Is(err, fs.ErrExist) || !strings.Contains(err.Error(), path) {
		t.Fatalf("WriteFileExclusive = %v, want fs.ErrExist naming %s", err, path)
	}
	if got, _ := os.ReadFile(path); string(got) != "x\n" {
		t.Fatalf("existing file changed to %q", got)
	}
	if names := dirEntries(t, dir); !reflect.DeepEqual(names, []string{"login-flow.md"}) {
		t.Fatalf("dir holds %v, want only login-flow.md", names)
	}
}

func TestReplaceFileAtomic_ExistingFile_ReplacesAsLFAndLeavesNoTempFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "login-flow.md")
	writeNodes(t, dir, "login-flow.md")
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatalf("Chmod: %v", err)
	}

	err := tree.ReplaceFileAtomic(path, strings.NewReader("new\r\ncontent\n"))

	if err != nil {
		t.Fatalf("ReplaceFileAtomic: %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "new\ncontent\n" {
		t.Fatalf("content = %q", got)
	}
	if info, err := os.Stat(path); err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0o644) {
		t.Fatalf("Stat = %v, %v; want mode 0644", info, err)
	}
	if names := dirEntries(t, dir); !reflect.DeepEqual(names, []string{"login-flow.md"}) {
		t.Fatalf("dir holds %v, want only login-flow.md", names)
	}
}

func TestReplaceFileAtomic_MissingPath_FailsNotExistCreatingNothing(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		// relativePath is the missing file, below a fresh empty directory.
		relativePath string
	}{
		{name: "missing file in an existing directory", relativePath: "login-flow.md"},
		{name: "missing directory", relativePath: filepath.Join("login-flow", "login-timeout-race.md")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			path := filepath.Join(dir, test.relativePath)

			err := tree.ReplaceFileAtomic(path, strings.NewReader("new\ncontent\n"))

			if !errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), path) {
				t.Fatalf("ReplaceFileAtomic = %v, want fs.ErrNotExist naming %s", err, path)
			}
			if names := dirEntries(t, dir); len(names) != 0 {
				t.Fatalf("dir holds %v, want nothing: no file, no directory, no temp file", names)
			}
		})
	}
}

// The copy fallback: on a file system that refuses hard links, WriteFileExclusive copies the temp
// file into path instead. These tests swap a package variable, so they never run in parallel.

func TestWriteFileExclusive_FreePathWithoutHardLinks_CopiesWholeFileAsLFAndLeavesNoTempFile(t *testing.T) {
	refused := refuseHardLinks(t)
	dir := filepath.Join(t.TempDir(), "login-flow")
	path := filepath.Join(dir, "login-timeout-race.md")
	content := strings.Repeat("retry the login\r\n", 4096) // past one 32 KiB copy buffer

	err := tree.WriteFileExclusive(path, strings.NewReader(content))

	if err != nil {
		t.Fatalf("WriteFileExclusive: %v", err)
	}
	if *refused != 1 {
		t.Fatalf("hard link tried %d times, want 1", *refused)
	}
	want := strings.ReplaceAll(content, "\r\n", "\n")
	if got, _ := os.ReadFile(path); string(got) != want {
		t.Fatalf("content = %d bytes, want the %d bytes written with LF endings", len(got), len(want))
	}
	if names := dirEntries(t, dir); !reflect.DeepEqual(names, []string{"login-timeout-race.md"}) {
		t.Fatalf("dir holds %v, want only login-timeout-race.md", names)
	}
}

func TestWriteFileExclusive_PathExistsWithoutHardLinks_FailsWithErrExistAndLeavesFileAndNoTempFile(t *testing.T) {
	refused := refuseHardLinks(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "login-flow.md")
	writeNodes(t, dir, "login-flow.md")

	err := tree.WriteFileExclusive(path, strings.NewReader("new\ncontent\n"))

	if !errors.Is(err, fs.ErrExist) || !strings.Contains(err.Error(), path) {
		t.Fatalf("WriteFileExclusive = %v, want fs.ErrExist naming %s", err, path)
	}
	if *refused != 1 {
		t.Fatalf("hard link tried %d times, want 1", *refused)
	}
	if got, _ := os.ReadFile(path); string(got) != "x\n" {
		t.Fatalf("existing file changed to %q", got)
	}
	if names := dirEntries(t, dir); !reflect.DeepEqual(names, []string{"login-flow.md"}) {
		t.Fatalf("dir holds %v, want only login-flow.md", names)
	}
}

// refuseHardLinks makes WriteFileExclusive's hard link fail as a file system without hard links
// fails it, with an error that is not fs.ErrExist, until t ends. It returns the count of links
// refused.
func refuseHardLinks(t *testing.T) *int {
	t.Helper()
	refused := 0
	tree.SetLink(t, func(oldPath string, newPath string) error {
		refused++
		return &os.LinkError{Op: "link", Old: oldPath, New: newPath, Err: errors.ErrUnsupported}
	})
	return &refused
}
