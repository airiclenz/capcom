package hooks

import (
	"bytes"
	"path/filepath"
	"slices"
	"strings"
)

// GitattributesPath is the .gitattributes file InstallMergeAttribute adds to, relative to the
// working tree root.
const GitattributesPath = ".gitattributes"

// mergeDriverAttribute is the .gitattributes line that hands issue files to the capcom-log merge
// driver.
const mergeDriverAttribute = ".capcom/issues/**/*.md merge=capcom-log"

// Line endings of a .gitattributes file.
const (
	lineEndingLF   = "\n"
	lineEndingCRLF = "\r\n"
)

// InstallMergeAttribute appends the merge driver line to the .gitattributes file in the working tree
// at worktreeRoot, creating the file when missing, and reports whether it wrote the file. A file that
// already holds the line is left as it is, so a re-run changes nothing. The file is read through
// nodefile.ReadRegularFile, so a symlink is never followed and a named pipe never blocks. An
// existing file keeps its bytes, its permissions and its line ending: it is replaced byte for byte
// through nodefile.ReplaceFileVerbatim, never through nodefile.WriteFileAtomic, which would turn
// CRLF into LF and reset the mode. A file that is not a regular file, is larger than 1 MiB, or
// cannot be read or replaced fails as *Error naming GitattributesPath; a failed write of a new file
// returns nodefile.WriteFileAtomic's error as is.
func InstallMergeAttribute(worktreeRoot string) (Status, error) {
	path := filepath.Join(worktreeRoot, GitattributesPath)
	content, isMissing, err := readWorktreeFile(path, GitattributesPath)
	if err != nil {
		return "", err
	}
	appended, isAdded := AppendMergeAttribute(content)
	if !isAdded {
		return Present, nil
	}
	if err := writeWorktreeFile(path, GitattributesPath, appended, isMissing); err != nil {
		return "", err
	}
	return Added, nil
}

// AppendMergeAttribute returns the .gitattributes content with the merge driver line appended,
// ended as the file's lines are: CRLF when the content holds a CRLF, LF otherwise. A last line
// without a line ending gets one first. Content that already holds the line, however its fields are
// spaced, comes back unchanged with isAdded false. content itself is never modified.
func AppendMergeAttribute(content []byte) (appended []byte, isAdded bool) {
	if hasMergeAttribute(content) {
		return content, false
	}
	ending := lineEndingLF
	if bytes.Contains(content, []byte(lineEndingCRLF)) {
		ending = lineEndingCRLF
	}
	appended = make([]byte, 0, len(content)+len(ending)+len(mergeDriverAttribute)+len(ending))
	appended = append(appended, content...)
	if len(content) > 0 && !bytes.HasSuffix(content, []byte(lineEndingLF)) {
		appended = append(appended, ending...)
	}
	appended = append(appended, mergeDriverAttribute...)
	return append(appended, ending...), true
}

// hasMergeAttribute reports whether the .gitattributes content holds the merge driver line, however
// its fields are spaced.
func hasMergeAttribute(content []byte) bool {
	want := strings.Fields(mergeDriverAttribute)
	for line := range strings.Lines(string(content)) {
		if slices.Equal(strings.Fields(line), want) {
			return true
		}
	}
	return false
}
