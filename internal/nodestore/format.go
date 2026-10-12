package nodestore

import (
	"bytes"

	"github.com/airiclenz/capcom/internal/tree"
)

// Formatted is the outcome of a Format: the node files it rewrote, or with isCheck would rewrite,
// and its warnings, each naming its file relative to the working tree root with forward slashes.
type Formatted struct {
	// Paths are the rewritten files, in tree order.
	Paths []string
	// Warnings are every tree problem, as a *tree.Error, then, file by file, a *FileError for each
	// file that does not parse and one for each validation problem of a file that does not validate.
	Warnings []error
}

// Format brings every node file the kind's tree places to its canonical form: a file whose bytes
// differ from the serialization of its parsed node is rewritten, unless isCheck, and listed either
// way. It never resolves an argument, so it works on a Kind without a Resolver.
//
// A tree problem, and a file that does not parse or validate, is a warning and the file is left
// alone. A tree that cannot be loaded at all fails as it is, its files relative to the working
// tree root; a node file that cannot be read, or that tree.ReadNodeFile refuses unread (not a
// regular file, or larger than 1 MiB), fails with a *FileError at it; a failed write is returned as
// it is. Files rewritten before a failure stay rewritten.
func (k *Kind[T]) Format(isCheck bool) (Formatted, error) {
	nodes, problems, err := k.load()
	if err != nil {
		return Formatted{}, err
	}
	var formatted Formatted
	for _, problem := range problems {
		formatted.Warnings = append(formatted.Warnings, k.relativeTreeError(problem))
	}

	for _, node := range nodes.Nodes() {
		path := k.relative(node.Path)
		data, err := tree.ReadNodeFile(node.Path)
		if err != nil {
			return Formatted{}, k.fileError(node.Path, err)
		}
		parsed, err := k.codec.Parse(path, data)
		if err != nil {
			formatted.Warnings = append(formatted.Warnings, k.fileError(node.Path, err))
			continue
		}
		if invalid := k.codec.Validate(path, parsed); len(invalid) > 0 {
			for _, problem := range invalid {
				formatted.Warnings = append(formatted.Warnings, k.fileError(node.Path, problem))
			}
			continue
		}
		if bytes.Equal(data, k.codec.Serialize(parsed)) {
			continue
		}
		if !isCheck {
			if _, err := k.write(node.Path, node.ID, parsed); err != nil {
				return Formatted{}, err
			}
		}
		formatted.Paths = append(formatted.Paths, path)
	}
	return formatted, nil
}
