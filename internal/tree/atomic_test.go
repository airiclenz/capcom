package tree_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
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
