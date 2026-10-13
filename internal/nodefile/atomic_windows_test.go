package nodefile_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/airiclenz/capcom/internal/nodefile"
)

// readerHoldTime is how long a test's reader keeps the target open: past the first rename and
// well inside the retries' backoff.
const readerHoldTime = 30 * time.Millisecond

// openReader writes content to path and returns it opened for reading, as another process holding
// the file would; Windows refuses to rename over it until it is closed.
func openReader(t *testing.T, path string, content string) *os.File {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	reader, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return reader
}

func TestWriteFileAtomic_ReaderHoldsTarget_RetriesUntilItCloses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "login-flow.md")
	reader := openReader(t, path, "old\n")
	closed := make(chan error, 1)
	go func() {
		time.Sleep(readerHoldTime)
		closed <- reader.Close()
	}()

	err := nodefile.WriteFileAtomic(path, strings.NewReader("new\ncontent\n"))

	if closeErr := <-closed; closeErr != nil {
		t.Fatalf("closing the reader: %v", closeErr)
	}
	if err != nil {
		t.Fatalf("WriteFileAtomic = %v, want success once the reader closes", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "new\ncontent\n" {
		t.Fatalf("file holds %q, want %q", got, "new\ncontent\n")
	}
}

func TestReplaceFileAtomic_TargetRemovedBeforeRetry_FailsNotExistCreatingNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "login-flow.md")
	reader := openReader(t, path, "old\n")
	var hookErr error
	nodefile.SetBeforeRenameRetry(t, func(target string) {
		if hookErr != nil {
			return
		}
		hookErr = errors.Join(reader.Close(), os.Remove(target))
	})

	err := nodefile.ReplaceFileAtomic(path, strings.NewReader("new\ncontent\n"))

	if hookErr != nil {
		t.Fatalf("closing and removing the target before the retry: %v", hookErr)
	}
	if !errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), path) {
		t.Fatalf("ReplaceFileAtomic = %v, want fs.ErrNotExist naming %s", err, path)
	}
	if names := dirEntries(t, dir); len(names) != 0 {
		t.Fatalf("dir holds %v, want nothing: no file and no temp file", names)
	}
}
