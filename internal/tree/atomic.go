package tree

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

// tempPattern names a temp file `.<file name>.tmp-<random>`, which Load skips as a dotfile.
const tempPattern = hiddenPrefix + "%s.tmp-*"

// WriteFileAtomic replaces the file at path with everything read from content, converting CRLF
// line endings to LF, and gives it mode 0644. It creates path's directory when missing, streams
// into a temp file in that directory, fsyncs it and renames it over path, then fsyncs the
// directory (off Windows). Readers see either the old file or the whole new one; on any failure
// the temp file is removed and the old file is left intact. Errors name path.
func WriteFileAtomic(path string, content io.Reader) error {
	return writeFileAtomic(path, content, true, filePermissions)
}

// WriteFileAtomicVerbatim replaces the file at path with everything read from content, byte for
// byte, and gives it the permission bits of mode. It writes as atomically as WriteFileAtomic and
// fails the same way; it is the write that puts a file back exactly as it was read.
func WriteFileAtomicVerbatim(path string, content io.Reader, mode fs.FileMode) error {
	return writeFileAtomic(path, content, false, mode.Perm())
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
// the file system refuses the link for any reason but an existing path, it copies the temp file
// into path opened with O_CREATE|O_EXCL instead. Readers see no file or the whole new one. When
// path already exists it fails with an error wrapping fs.ErrExist and leaves that file as it was;
// on any failure the temp file is removed. Errors name path.
func WriteFileExclusive(path string, content io.Reader) error {
	dir := filepath.Dir(path)
	temp, err := writeTemp(path, content, true, filePermissions)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	placeErr := placeExclusive(temp, path, filePermissions)
	removeErr := os.Remove(temp)
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

// writeFileAtomic is the atomic write of WriteFileAtomic and WriteFileAtomicVerbatim: it converts
// CRLF to LF when isConvertingCRLF and gives the file permissions perm.
func writeFileAtomic(path string, content io.Reader, isConvertingCRLF bool, perm fs.FileMode) error {
	dir := filepath.Dir(path)
	temp, err := writeTemp(path, content, isConvertingCRLF, perm)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := os.Rename(temp, path); err != nil {
		removeErr := os.Remove(temp)
		return fmt.Errorf("%s: %w", path, joinNonNil(err, removeErr))
	}
	if err := syncDir(dir); err != nil {
		return fmt.Errorf("%s: syncing directory: %w", path, err)
	}
	return nil
}

// writeTemp creates path's directory when missing and writes content, filled as fillTemp fills it,
// to a new closed temp file in that directory, whose name it returns. On any failure the temp file
// is removed.
func writeTemp(
	path string,
	content io.Reader,
	isConvertingCRLF bool,
	perm fs.FileMode,
) (string, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirPermissions); err != nil {
		return "", err
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

// placeExclusive gives the written file temp a second name, path, without replacing a file there:
// a hard link, or, when the file system refuses one for any other reason than an existing path,
// a copy through copyExclusive. An existing path fails with fs.ErrExist itself. temp stays behind.
func placeExclusive(temp string, path string, perm fs.FileMode) error {
	err := os.Link(temp, path)
	if err == nil {
		return nil
	}
	if errors.Is(err, fs.ErrExist) {
		return fs.ErrExist
	}
	// No hard links here (EPERM, ENOTSUP, Windows' ERROR_INVALID_FUNCTION on FAT): copy instead.
	return copyExclusive(temp, path, perm)
}

// copyExclusive copies the file temp into path, created with O_CREATE|O_EXCL and given the
// permissions perm, and fsyncs it. An existing path fails with fs.ErrExist itself; on any later
// failure the partly written path is removed.
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
