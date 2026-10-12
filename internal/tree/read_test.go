package tree_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/airiclenz/capcom/internal/tree"
)

func TestReadNodeFile_RegularFile_ReadsUpToTheLimit(t *testing.T) {
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

			data, err := tree.ReadNodeFile(path)

			if err != nil || !bytes.Equal(data, tc.data) {
				t.Errorf("ReadNodeFile = %d bytes, %v; want the %d bytes written", len(data), err, len(tc.data))
			}
		})
	}
}

func TestReadNodeFile_Refused_SentinelInPathError(t *testing.T) {
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
			want: tree.ErrNotRegular,
		},
		{name: "named pipe", make: makeFIFO, want: tree.ErrNotRegular},
		{
			name: "directory",
			make: func(t *testing.T, path string) {
				if err := os.Mkdir(path, 0o755); err != nil {
					t.Fatalf("Mkdir: %v", err)
				}
			},
			want: tree.ErrNotRegular,
		},
		{
			name: "1 MiB and one byte",
			make: func(t *testing.T, path string) { writeSizedFile(t, path, 1<<20+1) },
			want: tree.ErrTooLarge,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "node.md")
			tc.make(t, path)

			data, err := tree.ReadNodeFile(path)

			var pathErr *fs.PathError
			if data != nil || !errors.Is(err, tc.want) || !errors.As(err, &pathErr) || pathErr.Path != path {
				t.Errorf("ReadNodeFile = %d bytes, %v; want %v in an *fs.PathError naming %s", len(data), err, tc.want, path)
			}
		})
	}
}

func TestReadNodeFile_Missing_IsErrNotExist(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "gone.md")

	_, err := tree.ReadNodeFile(path)

	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadNodeFile = %v, want fs.ErrNotExist", err)
	}
}
