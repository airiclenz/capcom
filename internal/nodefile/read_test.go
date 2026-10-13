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

func TestReadRegularFile_RegularFile_ReadsUpToTheLimit(t *testing.T) {
	t.Parallel()
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

			data, err := nodefile.ReadRegularFile(path)

			if err != nil || !bytes.Equal(data, tc.data) {
				t.Errorf("ReadRegularFile = %d bytes, %v; want the %d bytes written",
					len(data), err, len(tc.data))
			}
		})
	}
}

// TestReadRegularFile_Refused_SentinelInPathError checks that a symlink, a named pipe, a directory
// and a file over 1 MiB are refused with the sentinel in an *fs.PathError naming the path.
func TestReadRegularFile_Refused_SentinelInPathError(t *testing.T) {
	t.Parallel()
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

			data, err := nodefile.ReadRegularFile(path)

			assertRefused(t, path, data, err, tc.want)
		})
	}
}

func TestReadRegularFile_Missing_IsErrNotExist(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "gone.md")

	_, err := nodefile.ReadRegularFile(path)

	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadRegularFile = %v, want fs.ErrNotExist", err)
	}
}

// assertRefused fails t unless ReadRegularFile returned no data and an error wrapping want in an
// *fs.PathError naming path.
func assertRefused(t *testing.T, path string, data []byte, err error, want error) {
	t.Helper()
	var pathErr *fs.PathError
	if data != nil || !errors.Is(err, want) || !errors.As(err, &pathErr) || pathErr.Path != path {
		t.Errorf("ReadRegularFile = %d bytes, %v; want %v in an *fs.PathError naming %s",
			len(data), err, want, path)
	}
}
