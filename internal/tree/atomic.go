package tree

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Permissions of the files and directories WriteFileAtomic creates.
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

// writeFileAtomic is the atomic write of WriteFileAtomic and WriteFileAtomicVerbatim: it converts
// CRLF to LF when isConvertingCRLF and gives the file permissions perm.
func writeFileAtomic(path string, content io.Reader, isConvertingCRLF bool, perm fs.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirPermissions); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	temp, err := os.CreateTemp(dir, fmt.Sprintf(tempPattern, filepath.Base(path)))
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	if err := fillTemp(temp, content, isConvertingCRLF, perm); err != nil {
		closeErr := temp.Close()
		removeErr := os.Remove(temp.Name())
		return fmt.Errorf("%s: %w", path, joinNonNil(err, closeErr, removeErr))
	}
	if err := temp.Close(); err != nil {
		removeErr := os.Remove(temp.Name())
		return fmt.Errorf("%s: %w", path, joinNonNil(err, removeErr))
	}
	if err := os.Rename(temp.Name(), path); err != nil {
		removeErr := os.Remove(temp.Name())
		return fmt.Errorf("%s: %w", path, joinNonNil(err, removeErr))
	}
	if err := syncDir(dir); err != nil {
		return fmt.Errorf("%s: syncing directory: %w", path, err)
	}
	return nil
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
