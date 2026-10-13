package nodefile_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/airiclenz/capcom/internal/nodefile"
)

// reader is a guarded reader under test: ReadRegularFile, or ReadNodeFile, which wraps it.
type reader func(path string) ([]byte, error)

func TestReadRegularFile_RegularFile_ReadsUpToTheLimit(t *testing.T) {
	t.Parallel()
	testReadsUpToTheLimit(t, "ReadRegularFile", nodefile.ReadRegularFile)
}

func TestReadRegularFile_Refused_SentinelInPathError(t *testing.T) {
	t.Parallel()
	testRefusedSentinelInPathError(t, "ReadRegularFile", nodefile.ReadRegularFile)
}

func TestReadNodeFile_RegularFile_ReadsUpToTheLimit(t *testing.T) {
	t.Parallel()
	testReadsUpToTheLimit(t, "ReadNodeFile", nodefile.ReadNodeFile)
}

func TestReadNodeFile_Refused_SentinelInPathError(t *testing.T) {
	t.Parallel()
	testRefusedSentinelInPathError(t, "ReadNodeFile", nodefile.ReadNodeFile)
}

func TestReadNodeFile_Missing_IsErrNotExist(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "gone.md")

	_, err := nodefile.ReadNodeFile(path)

	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadNodeFile = %v, want fs.ErrNotExist", err)
	}
}

// testReadsUpToTheLimit checks that read, called name in failures, returns a regular file's bytes
// up to and including 1 MiB.
func testReadsUpToTheLimit(t *testing.T, name string, read reader) {
	t.Helper()
	cases := []struct {
		name string
		data []byte
	}{
		{name: "small", data: []byte("---\ntitle: x\n---\n")},
		{name: "exactly 1 MiB", data: bytes.Repeat([]byte("x"), 1<<20)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "node.md")
			if err := os.WriteFile(path, tc.data, 0o644); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}

			data, err := read(path)

			if err != nil || !bytes.Equal(data, tc.data) {
				t.Errorf("%s = %d bytes, %v; want the %d bytes written", name, len(data), err, len(tc.data))
			}
		})
	}
}

// testRefusedSentinelInPathError checks that read, called name in failures, refuses a symlink, a
// named pipe, a directory and a file over 1 MiB with the sentinel in an *fs.PathError naming the
// path.
func testRefusedSentinelInPathError(t *testing.T, name string, read reader) {
	t.Helper()
	cases := []struct {
		name string
		make func(t *testing.T, path string)
		want error
	}{
		{
			name: "symlink to a regular file",
			make: func(t *testing.T, path string) {
				target := filepath.Join(t.TempDir(), "target.md")
				writeSizedFile(t, target, 1)
				makeSymlink(t, target, path)
			},
			want: nodefile.ErrNotRegular,
		},
		{name: "named pipe", make: makeFIFO, want: nodefile.ErrNotRegular},
		{
			name: "directory",
			make: func(t *testing.T, path string) {
				if err := os.Mkdir(path, 0o755); err != nil {
					t.Fatalf("Mkdir: %v", err)
				}
			},
			want: nodefile.ErrNotRegular,
		},
		{
			name: "1 MiB and one byte",
			make: func(t *testing.T, path string) { writeSizedFile(t, path, 1<<20+1) },
			want: nodefile.ErrTooLarge,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "node.md")
			tc.make(t, path)

			data, err := read(path)

			assertRefused(t, name, path, data, err, tc.want)
		})
	}
}

// assertRefused fails t unless the read called name returned no data and an error wrapping want in
// an *fs.PathError naming path.
func assertRefused(t *testing.T, name string, path string, data []byte, err error, want error) {
	t.Helper()
	var pathErr *fs.PathError
	if data != nil || !errors.Is(err, want) || !errors.As(err, &pathErr) || pathErr.Path != path {
		t.Errorf("%s = %d bytes, %v; want %v in an *fs.PathError naming %s", name, len(data), err, want, path)
	}
}
