package gitx

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// unknownObject is an object name no test repository holds.
const unknownObject = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

// blobOf returns the object name of the file at path in HEAD.
func blobOf(t *testing.T, root, path string) string {
	t.Helper()
	return revParse(t, root, "HEAD:"+path)
}

// corruptBody rewrites the loose object of blob so git still reads its header but fails part way
// through its content.
func corruptBody(t *testing.T, root, blob string) {
	t.Helper()
	path := filepath.Join(root, ".git", "objects", blob[:2], blob[2:])
	compressed, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	inflater, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("zlib.NewReader: %v", err)
	}
	object, err := io.ReadAll(inflater)
	if err != nil {
		t.Fatalf("inflate: %v", err)
	}
	var deflated bytes.Buffer
	deflater := zlib.NewWriter(&deflated)
	if _, err := deflater.Write(object); err != nil {
		t.Fatalf("deflate: %v", err)
	}
	if err := deflater.Close(); err != nil {
		t.Fatalf("deflate: %v", err)
	}
	corrupt := deflated.Bytes()
	for i := len(corrupt) / 2; i < len(corrupt); i++ {
		corrupt[i] ^= 0xff
	}
	// git writes loose objects read-only, which Windows refuses to overwrite.
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	if err := os.WriteFile(path, corrupt, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestReadBlobs_SeveralBlobs_ReadsEachInOneCall(t *testing.T) {
	root := newRepo(t)
	commitFile(t, root, "issues/a.md", "first\n")
	commitFile(t, root, "issues/b.md", "second\n")
	commitFile(t, root, "issues/empty.md", "")
	first, second, empty := blobOf(t, root, "issues/a.md"), blobOf(t, root, "issues/b.md"),
		blobOf(t, root, "issues/empty.md")

	blobs, err := ReadBlobs(root, []string{first, second, empty, first}, showFileLimit)

	if err != nil || len(blobs) != 3 {
		t.Fatalf("ReadBlobs = %v, %v; want three blobs", blobs, err)
	}
	for object, want := range map[string]string{first: "first\n", second: "second\n", empty: ""} {
		if blob := blobs[object]; blob.Err != nil || string(blob.Data) != want {
			t.Errorf("blob %s = %q, %v; want %q", object, blob.Data, blob.Err, want)
		}
	}
}

func TestReadBlobs_BlobOverTheLimit_GetsErrBlobTooLargeAndTheNextStillReads(t *testing.T) {
	const limit = 8
	root := newRepo(t)
	commitFile(t, root, "issues/big.md", "longer than eight bytes\n")
	commitFile(t, root, "issues/small.md", "small\n")
	big, small := blobOf(t, root, "issues/big.md"), blobOf(t, root, "issues/small.md")

	blobs, err := ReadBlobs(root, []string{big, small}, limit)

	if err != nil {
		t.Fatalf("ReadBlobs: %v", err)
	}
	if blob := blobs[big]; !errors.Is(blob.Err, ErrBlobTooLarge) || blob.Data != nil {
		t.Errorf("big blob = %q, %v; want no content and ErrBlobTooLarge", blob.Data, blob.Err)
	}
	if blob := blobs[small]; blob.Err != nil || string(blob.Data) != "small\n" {
		t.Errorf("small blob = %q, %v; want its content", blob.Data, blob.Err)
	}
}

func TestReadBlobs_UnknownObject_GetsErrBlobMissing(t *testing.T) {
	root := newRepo(t)
	commitFile(t, root, "issues/a.md", "first\n")
	known := blobOf(t, root, "issues/a.md")

	blobs, err := ReadBlobs(root, []string{unknownObject, known}, showFileLimit)

	if err != nil {
		t.Fatalf("ReadBlobs: %v", err)
	}
	if blob := blobs[unknownObject]; !errors.Is(blob.Err, ErrBlobMissing) {
		t.Errorf("unknown blob error = %v, want ErrBlobMissing", blob.Err)
	}
	if blob := blobs[known]; blob.Err != nil || string(blob.Data) != "first\n" {
		t.Errorf("known blob = %q, %v; want its content", blob.Data, blob.Err)
	}
}

func TestReadBlobs_EmptyList_StartsNoProcess(t *testing.T) {
	// A process started in a directory that does not exist would fail to start.
	missingDir := filepath.Join(t.TempDir(), "absent")

	blobs, err := ReadBlobs(missingDir, nil, showFileLimit)

	if err != nil || len(blobs) != 0 {
		t.Errorf("ReadBlobs = %v, %v; want none and no error", blobs, err)
	}
}

func TestReadBlobs_NotARepository_ReturnsTheGitError(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	setHermeticGitEnv(t, dir)

	blobs, err := ReadBlobs(dir, []string{unknownObject}, showFileLimit)

	var gitFailure *Error
	if !errors.As(err, &gitFailure) || blobs != nil {
		t.Errorf("ReadBlobs = %v, %v; want no blobs and a git error", blobs, err)
	}
}

func TestReadBlobs_BodyCorruptBlobBeforeAGoodOne_GetsGitsStderrAndTheGoodOneReads(t *testing.T) {
	root := newRepo(t)
	var content strings.Builder
	for i := range 4096 {
		fmt.Fprintf(&content, "line %d holds %d\n", i, i*7919%100003)
	}
	commitFile(t, root, "issues/corrupt.md", content.String())
	commitFile(t, root, "issues/good.md", "good\n")
	corrupt, good := blobOf(t, root, "issues/corrupt.md"), blobOf(t, root, "issues/good.md")
	corruptBody(t, root, corrupt)

	blobs, err := ReadBlobs(root, []string{corrupt, good}, showFileLimit)

	if err != nil {
		t.Fatalf("ReadBlobs: %v", err)
	}
	var gitFailure *Error
	broken := blobs[corrupt]
	hasStderr := errors.As(broken.Err, &gitFailure) && strings.TrimSpace(gitFailure.Stderr) != ""
	if !hasStderr || broken.Data != nil {
		t.Errorf("corrupt blob = %q, %v; want no content and git's stderr", broken.Data, broken.Err)
	}
	if blob := blobs[good]; blob.Err != nil || string(blob.Data) != "good\n" {
		t.Errorf("good blob = %q, %v; want its content", blob.Data, blob.Err)
	}
}
