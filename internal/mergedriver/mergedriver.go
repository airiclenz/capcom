// Package mergedriver merges three versions of an Issue file the way capcom's git merge driver
// does: the Log by entry, the rest of the file by git's text merge, with the result replacing ours.
// It decides from the files alone; the wording of a failure or a conflict is the caller's.
package mergedriver

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"

	"github.com/airiclenz/capcom/internal/gitx"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/nodefile"
)

// Of the text merge's temp inputs.
const (
	// tempDirPattern names the temp directory that holds the text merge's inputs.
	tempDirPattern = "capcom-merge-*"
	// inputPermissions are the permissions of the text merge's temp input files.
	inputPermissions = 0o600
)

// Positions of the three versions in a merge's inputs.
const (
	inputBase = iota
	inputOurs
	inputTheirs
)

// Merge merges the Issue file versions at basePath, oursPath and theirsPath and writes the result
// to oursPath, byte for byte: the Log merges by entry, everything else with git merge-file under
// labels. It reports whether conflict markers remain in what it wrote. On failure it writes
// nothing, leaving ours as it was, and returns the error with failedPath naming the input it could
// not read or the ours it could not write; failedPath is empty when the merge itself failed.
func Merge(basePath, oursPath, theirsPath string, labels gitx.MergeLabels) (hasConflict bool, failedPath string, err error) {
	inputs := []string{basePath, oursPath, theirsPath}
	versions := make([][]byte, 0, len(inputs))
	for _, input := range inputs {
		content, err := os.ReadFile(input)
		if err != nil {
			return false, input, err
		}
		versions = append(versions, content)
	}

	merged, hasConflict, err := issue.MergeFile(
		versions[inputBase],
		versions[inputOurs],
		versions[inputTheirs],
		gitTextMerge(labels),
	)
	if err != nil {
		return false, "", err
	}
	if err := nodefile.ReplaceFileVerbatim(oursPath, bytes.NewReader(merged)); err != nil {
		return false, oursPath, err
	}
	return hasConflict, "", nil
}

// gitTextMerge returns the text merge issue.MergeFile runs on everything outside the Log: it
// writes the three versions to a temp directory, merges them with git merge-file under labels and
// removes the directory again.
func gitTextMerge(labels gitx.MergeLabels) issue.TextMerge {
	return func(base []byte, ours []byte, theirs []byte) (merged []byte, hasConflict bool, err error) {
		dir, err := os.MkdirTemp("", tempDirPattern)
		if err != nil {
			return nil, false, err
		}
		defer func() {
			err = errors.Join(err, os.RemoveAll(dir))
		}()

		versions := [][]byte{base, ours, theirs}
		paths := make([]string, 0, len(versions))
		for index, content := range versions {
			path := filepath.Join(dir, strconv.Itoa(index))
			if err := os.WriteFile(path, content, inputPermissions); err != nil {
				return nil, false, err
			}
			paths = append(paths, path)
		}
		return gitx.MergeFile(paths[inputBase], paths[inputOurs], paths[inputTheirs], labels)
	}
}
