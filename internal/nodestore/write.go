package nodestore

import (
	"bytes"
	"errors"
	"io/fs"
	"strings"

	"github.com/airiclenz/capcom/internal/frontmatter"
	"github.com/airiclenz/capcom/internal/nodefile"
)

// problemSeparator joins the validation problems of one node in a single message.
const problemSeparator = "; "

// writeMode says whether write may replace a file already at the node's path.
type writeMode int

const (
	// replaceFile replaces the existing file atomically, as Edit and Format do, and never creates
	// a missing file or directory.
	replaceFile writeMode = iota
	// createFile never replaces a file: one already at the path fails with an error wrapping
	// fs.ErrExist, as Create needs.
	createFile
)

// write validates node and writes it, with LF line endings, to path, the absolute file of the node
// id: under replaceFile replacing the file there atomically, failing with a *FileError at it when
// the file or its directory is missing, which it never creates; under createFile only where no
// file is, failing with an error wrapping fs.ErrExist otherwise. A node that does not validate
// fails with a *FileError listing every problem, citing the first problem's line, and is not
// written.
func (k *Kind[T]) write(path string, id string, node T, mode writeMode) (Written, error) {
	data, err := k.encode(path, node)
	if err != nil {
		return Written{}, err
	}
	return k.place(path, id, data, mode)
}

// encode validates node, to be written to path, the absolute file of the node, and returns its
// serialized bytes with LF line endings; a node that does not validate fails as write documents.
func (k *Kind[T]) encode(path string, node T) ([]byte, error) {
	relative := k.relative(path)
	if problems := k.codec.Validate(relative, node); len(problems) > 0 {
		return nil, validationError(relative, problems)
	}
	return bytes.ReplaceAll(k.codec.Serialize(node), []byte("\r\n"), []byte("\n")), nil
}

// place writes data, the bytes encode returned, to path, the absolute file of the node id, under
// mode as write documents, and returns the node written.
func (k *Kind[T]) place(path string, id string, data []byte, mode writeMode) (Written, error) {
	writeFile := nodefile.ReplaceFileAtomic
	if mode == createFile {
		writeFile = nodefile.WriteFileExclusive
	}
	err := writeFile(path, bytes.NewReader(data))
	if mode == replaceFile && errors.Is(err, fs.ErrNotExist) {
		// Another session moved or removed the node file since it was read.
		return Written{}, k.fileError(path, err)
	}
	if err != nil {
		return Written{}, err
	}
	return Written{ID: id, Path: k.relative(path), ETag: nodefile.ETag(data)}, nil
}

// validationError is the *FileError of a node at path, relative to the working tree root, that
// does not validate: every problem's message, at the first problem's line, marked IsValidation.
func validationError(path string, problems []*frontmatter.Error) *FileError {
	messages := make([]string, 0, len(problems))
	for _, problem := range problems {
		messages = append(messages, problem.Message)
	}
	return &FileError{
		Path:         path,
		Line:         problems[0].Line,
		Message:      strings.Join(messages, problemSeparator),
		IsValidation: true,
	}
}
