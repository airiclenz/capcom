package nodefile

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Permissions of the files and directories WriteFileAtomic and WriteFileExclusive create.
const (
	filePermissions = 0o644
	dirPermissions  = 0o755
)

// hiddenPrefix starts the name of every temp file, so tree's Load skips it as a dotfile.
const hiddenPrefix = "."

// tempPattern names a temp file `.<file name>.tmp-<random>`, which Load skips as a dotfile.
const tempPattern = hiddenPrefix + "%s.tmp-*"

// link is the hard link placeExclusive names the written file with; a test swaps it to force the
// no-replace rename.
var link = os.Link

// renameNoReplace is the no-replace rename placeExclusive moves the written file into place with
// when the file system refuses the hard link; a test swaps it to count its calls or to force the
// in-place copy.
var renameNoReplace = noReplaceRename

// Link gives the file oldPath the second name newPath with the hard link WriteFileExclusive names
// its file with, which the file system refuses atomically, with an error wrapping fs.ErrExist,
// when newPath exists. It is os.Link unless a test swapped it with SwapLink.
func Link(oldPath string, newPath string) error {
	return link(oldPath, newPath)
}

// SwapLink replaces the hard link Link and WriteFileExclusive use with fn and returns the function
// that puts the previous one back. It is a test hook for the packages whose tests force a hard
// link to fail: it swaps a package variable, so a test that calls it never runs in parallel.
func SwapLink(fn func(oldPath string, newPath string) error) (restore func()) {
	saved := link
	link = fn
	return func() { link = saved }
}

// WriteFileAtomic replaces the file at path with everything read from content, converting CRLF
// line endings to LF, and gives it mode 0644. It creates path's directory when missing, streams
// into a temp file in that directory, fsyncs it and renames it over path, then fsyncs the
// directory (off Windows). On Windows, where a reader holding path open blocks the rename, the
// rename is tried up to five times over about 150 ms before its last error is returned. Readers
// see either the old file or the whole new one; on any failure the temp file is removed and the
// old file is left intact. Errors name path.
func WriteFileAtomic(path string, content io.Reader) error {
	return writeFileAtomic(path, content, true, filePermissions, false)
}

// ReplaceFileAtomic replaces the existing file at path with everything read from content, as
// WriteFileAtomic does: CRLF line endings become LF, the file gets mode 0644 and readers see either
// the old file or the whole new one. Unlike WriteFileAtomic it never creates a file or directory,
// so a node file another session moved or removed is not brought back: when path is missing just
// before the rename, or before a Windows retry of it, it fails with an error wrapping
// fs.ErrNotExist, and a missing directory fails the same way. On any failure the temp file is removed. Errors name path.
func ReplaceFileAtomic(path string, content io.Reader) error {
	return writeFileAtomic(path, content, true, filePermissions, true)
}

// WriteFileAtomicVerbatim replaces the file at path with everything read from content, byte for
// byte, and gives it the permission bits of mode. It writes as atomically as WriteFileAtomic and
// fails the same way; it is the write that puts a file back exactly as it was read.
func WriteFileAtomicVerbatim(path string, content io.Reader, mode fs.FileMode) error {
	return writeFileAtomic(path, content, false, mode.Perm(), false)
}

// ReplaceFileVerbatim replaces the existing file at path with everything read from content, byte
// for byte, keeping the old file's permission bits: CRLF line endings and the mode survive, unlike
// through WriteFileAtomic. It writes as WriteFileAtomicVerbatim does, and fails when path is
// missing. Errors name path.
func ReplaceFileVerbatim(path string, content io.Reader) error {
	info, err := os.Stat(path)
	if err != nil {
		return err // a *fs.PathError, which names path
	}
	return WriteFileAtomicVerbatim(path, content, info.Mode())
}

// WriteFileExclusive creates the file at path with everything read from content, converting CRLF
// line endings to LF, and gives it mode 0644; unlike WriteFileAtomic it never replaces a file. It
// creates path's directory when missing, streams into a temp file in that directory and fsyncs it,
// hard-links the temp file to path and removes it, then fsyncs the directory (off Windows). Where
// the file system refuses the link for any reason but an existing path, it renames the temp file
// to path with a rename that refuses an existing path (renameat2 RENAME_NOREPLACE on Linux,
// renamex_np RENAME_EXCL on macOS, MoveFileEx without MOVEFILE_REPLACE_EXISTING on Windows).
// Either way readers see no file or the whole new one. Only where the platform has no such rename
// or the file system refuses it for any reason but an existing path does it copy the temp file
// into path opened with O_CREATE|O_EXCL; there a reader can see the file part-written, and a crash
// can leave it so. When path already exists it fails with an error wrapping fs.ErrExist and
// leaves that file as it was; on any failure the temp file is removed. Errors name path.
func WriteFileExclusive(path string, content io.Reader) error {
	dir := filepath.Dir(path)
	temp, err := writeTemp(path, content, true, filePermissions, true)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	isTempRenamed, placeErr := placeExclusive(temp, path, filePermissions)
	var removeErr error
	if !isTempRenamed {
		removeErr = os.Remove(temp)
	}
	if placeErr != nil {
		return fmt.Errorf("%s: %w", path, joinNonNil(placeErr, removeErr))
	}
	if removeErr != nil {
		return fmt.Errorf("%s: written, but removing the temp file: %w", path, removeErr)
	}
	if err := syncDir(dir); err != nil {
		return fmt.Errorf("%s: syncing directory: %w", path, err)
	}
	return nil
}

// writeFileAtomic is the atomic write of WriteFileAtomic, WriteFileAtomicVerbatim and
// ReplaceFileAtomic: it converts CRLF to LF when isConvertingCRLF and gives the file permissions
// perm. It renames through replaceRename, which retries on Windows while a reader holds path.
// When isReplacing it creates no directory and renames only over a file still at path, checked
// before the rename and every retry, failing with a *fs.PathError (Op "replace") wrapping
// fs.ErrNotExist when path is gone.
func writeFileAtomic(
	path string,
	content io.Reader,
	isConvertingCRLF bool,
	perm fs.FileMode,
	isReplacing bool,
) error {
	dir := filepath.Dir(path)
	temp, err := writeTemp(path, content, isConvertingCRLF, perm, !isReplacing)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if isReplacing {
		if err := checkReplaceTarget(path); err != nil {
			removeErr := os.Remove(temp)
			return joinNonNil(err, removeErr)
		}
	}
	if err := replaceRename(temp, path, isReplacing); err != nil {
		removeErr := os.Remove(temp)
		return joinNonNil(err, removeErr)
	}
	if err := syncDir(dir); err != nil {
		return fmt.Errorf("%s: syncing directory: %w", path, err)
	}
	return nil
}

// checkReplaceTarget fails with a *fs.PathError (Op "replace") naming path that wraps
// fs.ErrNotExist when nothing is at path; any other Lstat failure is returned as it is.
func checkReplaceTarget(path string) error {
	_, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &fs.PathError{Op: "replace", Path: path, Err: fs.ErrNotExist}
	}
	return err
}

// writeTemp writes content, filled as fillTemp fills it, to a new closed temp file in path's
// directory, whose name it returns; it creates that directory first when isCreatingDir and it is
// missing. On any failure the temp file is removed.
func writeTemp(
	path string,
	content io.Reader,
	isConvertingCRLF bool,
	perm fs.FileMode,
	isCreatingDir bool,
) (string, error) {
	dir := filepath.Dir(path)
	if isCreatingDir {
		if err := os.MkdirAll(dir, dirPermissions); err != nil {
			return "", err
		}
	}
	temp, err := os.CreateTemp(dir, fmt.Sprintf(tempPattern, filepath.Base(path)))
	if err != nil {
		return "", err
	}

	if err := fillTemp(temp, content, isConvertingCRLF, perm); err != nil {
		closeErr := temp.Close()
		removeErr := os.Remove(temp.Name())
		return "", joinNonNil(err, closeErr, removeErr)
	}
	if err := temp.Close(); err != nil {
		removeErr := os.Remove(temp.Name())
		return "", joinNonNil(err, removeErr)
	}
	return temp.Name(), nil
}

// placeExclusive puts the written file temp at path without replacing a file there, trying in
// turn: a hard link, which leaves temp behind; when the file system refuses one for any other
// reason than an existing path, a no-replace rename of temp, which consumes it and reports
// isTempRenamed; when the platform has no such rename or the file system refuses it for any other
// reason than an existing path, a copy through copyExclusive, which leaves temp behind. The link
// and the rename place the whole file atomically; only the copy can be seen part-written. An
// existing path fails with fs.ErrExist itself, and temp stays behind.
func placeExclusive(temp string, path string, perm fs.FileMode) (isTempRenamed bool, err error) {
	err = link(temp, path)
	if err == nil {
		return false, nil
	}
	if errors.Is(err, fs.ErrExist) {
		return false, fs.ErrExist
	}

	// No hard links here (EPERM, ENOTSUP, Windows' ERROR_INVALID_FUNCTION on FAT): rename instead.
	err = renameNoReplace(temp, path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrExist) {
		return false, fs.ErrExist
	}
	// No no-replace rename here (EINVAL, ENOSYS, ENOTSUP, another platform): copy in place.
	return false, copyExclusive(temp, path, perm)
}

// copyExclusive copies the file temp into path, created with O_CREATE|O_EXCL and given the
// permissions perm, and fsyncs it. It is placeExclusive's last resort: path exists from the
// create on, so a reader can see it part-written and a crash can leave it so. An existing path
// fails with fs.ErrExist itself; on any later failure the partly written path is removed.
func copyExclusive(temp string, path string, perm fs.FileMode) error {
	source, err := os.Open(temp)
	if err != nil {
		return err
	}
	target, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		closeErr := source.Close()
		if errors.Is(err, fs.ErrExist) {
			err = fs.ErrExist
		}
		return joinNonNil(err, closeErr)
	}

	fillErr := fillTemp(target, source, false, perm)
	targetCloseErr := target.Close()
	sourceCloseErr := source.Close()
	if fillErr == nil && targetCloseErr == nil {
		return sourceCloseErr
	}
	removeErr := os.Remove(path)
	if fillErr == nil {
		return joinNonNil(targetCloseErr, sourceCloseErr, removeErr)
	}
	return joinNonNil(fillErr, targetCloseErr, sourceCloseErr, removeErr)
}

// fillTemp copies content into temp, with LF line endings when isConvertingCRLF, sets the
// permissions perm and fsyncs.
func fillTemp(temp *os.File, content io.Reader, isConvertingCRLF bool, perm fs.FileMode) error {
	if err := copyContent(temp, content, isConvertingCRLF); err != nil {
		return err
	}
	if err := temp.Chmod(perm); err != nil {
		return err
	}
	return temp.Sync()
}

// copyContent copies content into target, with LF line endings when isConvertingCRLF.
func copyContent(target io.Writer, content io.Reader, isConvertingCRLF bool) error {
	if !isConvertingCRLF {
		_, err := io.Copy(target, content)
		return err
	}
	writer := &lfWriter{target: target}
	if _, err := io.Copy(writer, content); err != nil {
		return err
	}
	return writer.flush()
}

// joinNonNil returns the first error wrapped, with any later cleanup errors appended to its text.
func joinNonNil(first error, cleanup ...error) error {
	for _, err := range cleanup {
		if err != nil {
			first = fmt.Errorf("%w (cleanup: %v)", first, err)
		}
	}
	return first
}

// lfWriter forwards writes to target with every CRLF pair turned into LF. A CR at the end of one
// write is held back until the next byte (or flush) shows whether an LF follows.
type lfWriter struct {
	target      io.Writer
	isPendingCR bool
}

// Write converts p and writes it on; it reports len(p) on success.
func (w *lfWriter) Write(p []byte) (int, error) {
	converted := make([]byte, 0, len(p)+1)
	for _, b := range p {
		if w.isPendingCR && b != '\n' {
			converted = append(converted, '\r')
		}
		w.isPendingCR = b == '\r'
		if !w.isPendingCR {
			converted = append(converted, b)
		}
	}
	if _, err := w.target.Write(converted); err != nil {
		return 0, err
	}
	return len(p), nil
}

// flush writes a CR still held back at the end of the content.
func (w *lfWriter) flush() error {
	if !w.isPendingCR {
		return nil
	}
	w.isPendingCR = false
	_, err := w.target.Write([]byte{'\r'})
	return err
}
