package nodefile_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/nodefile"
)

// The FS every Disk test reads and writes through, so Disk is checked as the interface callers see.
var disk nodefile.FS = nodefile.Disk{}

func TestDiskStatAndLstat_Symlink_StatFollowsLstatDoesNot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "target.md")
	writeSizedFile(t, target, 3)
	link := filepath.Join(dir, "link.md")
	makeSymlink(t, target, link)

	followed, statErr := disk.Stat(link)
	own, lstatErr := disk.Lstat(link)

	if statErr != nil || !followed.Mode().IsRegular() {
		t.Errorf("Stat(link) = %v, %v; want the regular target", followed, statErr)
	}
	if lstatErr != nil || own.Mode()&fs.ModeSymlink == 0 {
		t.Errorf("Lstat(link) = %v, %v; want the symlink itself", own, lstatErr)
	}
}

func TestDiskStatAndLstat_Missing_IsErrNotExist(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "missing.md")

	_, statErr := disk.Stat(missing)
	_, lstatErr := disk.Lstat(missing)

	if !errors.Is(statErr, fs.ErrNotExist) || !errors.Is(lstatErr, fs.ErrNotExist) {
		t.Errorf("Stat, Lstat of a missing path = %v, %v; want fs.ErrNotExist", statErr, lstatErr)
	}
}

func TestDiskReadDir_Directory_ListsEntriesByName(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNodes(t, dir, "b.md", "a.md", "a/c.md")

	entries, err := disk.ReadDir(dir)

	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if want := []string{"a", "a.md", "b.md"}; !reflect.DeepEqual(names, want) {
		t.Errorf("ReadDir names = %v, want %v", names, want)
	}
}

func TestDiskReadRegularFile_GuardedReader_ReadsRegularRefusesTooLarge(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	small := filepath.Join(dir, "small.md")
	writeSizedFile(t, small, 5)
	large := filepath.Join(dir, "large.md")
	writeSizedFile(t, large, nodefile.MaxNodeFileSize+1)

	data, smallErr := disk.ReadRegularFile(small)
	_, largeErr := disk.ReadRegularFile(large)

	if smallErr != nil || len(data) != 5 {
		t.Errorf("ReadRegularFile(small) = %d bytes, %v; want 5 bytes", len(data), smallErr)
	}
	if !errors.Is(largeErr, nodefile.ErrTooLarge) {
		t.Errorf("ReadRegularFile(large) error = %v, want ErrTooLarge", largeErr)
	}
}

func TestDiskReplaceFile_ExistingFile_ReplacedWithLFEndings(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "node.md")
	writeSizedFile(t, path, 3)

	err := disk.ReplaceFile(path, strings.NewReader("new\r\n"))

	if err != nil {
		t.Fatalf("ReplaceFile: %v", err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "new\n" {
		t.Errorf("file after ReplaceFile = %q, %v; want %q", data, err, "new\n")
	}
}

func TestDiskReplaceFile_Missing_IsErrNotExistAndCreatesNothing(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "node.md")

	err := disk.ReplaceFile(path, strings.NewReader("new\n"))

	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReplaceFile of a missing file = %v, want fs.ErrNotExist", err)
	}
	if _, statErr := os.Lstat(path); !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("Lstat after the failed ReplaceFile = %v, want fs.ErrNotExist", statErr)
	}
}

func TestDiskCreateFile_NewFile_CreatedWithItsDirectory(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "parent", "node.md")

	err := disk.CreateFile(path, strings.NewReader("new\r\n"))

	if err != nil {
		t.Fatalf("CreateFile: %v", err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "new\n" {
		t.Errorf("file after CreateFile = %q, %v; want %q", data, err, "new\n")
	}
}

func TestDiskCreateFile_ExistingFile_IsErrExistAndLeavesIt(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "node.md")
	if err := os.WriteFile(path, []byte("old\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	err := disk.CreateFile(path, strings.NewReader("new\n"))

	if !errors.Is(err, fs.ErrExist) {
		t.Errorf("CreateFile over an existing file = %v, want fs.ErrExist", err)
	}
	if data, readErr := os.ReadFile(path); readErr != nil || string(data) != "old\n" {
		t.Errorf("file after the failed CreateFile = %q, %v; want %q", data, readErr, "old\n")
	}
}
