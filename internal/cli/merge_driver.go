package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/gitx"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/tree"
)

// Arguments and output of capcom merge-driver.
const (
	kindMerge = "merge"
	// mergeDriverMinArgs are base, ours, theirs and path; up to three labels may follow.
	mergeDriverMinArgs = 4
	mergeDriverMaxArgs = 7
	// mergeTempDirPattern names the temp directory that holds the text merge's inputs.
	mergeTempDirPattern = "capcom-merge-*"
	// conflictMessage is the failure of a merge that leaves conflict markers in ours.
	conflictMessage = "conflict markers remain; resolve them and stage the file"
	// mergeInputPermissions are the permissions of the text merge's temp input files.
	mergeInputPermissions = 0o600
)

// Positions of the merge driver's arguments: git's %O %A %B %P %S %X %Y.
const (
	mergeArgBase = iota
	mergeArgOurs
	mergeArgTheirs
	mergeArgPath
	mergeArgBaseLabel
	mergeArgOursLabel
	mergeArgTheirsLabel
)

// unexpandedLabels are the label placeholders a git older than 2.44 passes through literally.
var unexpandedLabels = []string{"%S", "%X", "%Y"}

// The labels capcom puts after the conflict markers when git passes none, each after the path.
const (
	fallbackBaseSuffix   = " (base)"
	fallbackOursSuffix   = " (ours)"
	fallbackTheirsSuffix = " (theirs)"
)

// mergeDriverData is the --json data of a merge-driver run that merged cleanly.
type mergeDriverData struct {
	Path string `json:"path"`
}

// newMergeDriverCommand is the git merge driver for issue files. It merges the issue's Log by entry
// and the rest of the file with git merge-file, writes the result to <ours> and fails, exit 1, when
// conflict markers remain. It reads no store and no index, so it works outside a capcom store.
func (a *app) newMergeDriverCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "merge-driver <base> <ours> <theirs> <path> [<base-label> <ours-label> <theirs-label>]",
		Short: "Merge three versions of an issue file, its Log by entry (a git merge driver)",
		Long: "Merge three versions of an issue file as a git merge driver: " +
			"`capcom merge-driver %O %A %B %P %S %X %Y`. The Log merges by entry; the rest of the file " +
			"merges with git merge-file. The result replaces <ours>; the exit code is 1 when conflict " +
			"markers remain.",
		Args: cobra.RangeArgs(mergeDriverMinArgs, mergeDriverMaxArgs),
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			return a.emit(cmd, mergeIssueFile(args))
		}),
	}
}

// mergeIssueFile runs one merge-driver invocation on args and returns its result: a failure when
// an input cannot be read, the merge fails or ours cannot be written (ours is then left as it was),
// and a conflict failure, with ours written, when conflict markers remain.
func mergeIssueFile(args []string) Result {
	path := args[mergeArgPath]
	inputs := args[:mergeArgPath]
	versions := make([][]byte, 0, len(inputs))
	for _, input := range inputs {
		content, err := os.ReadFile(input)
		if err != nil {
			return failedMerge(&Error{Code: CodeInvalid, Path: input, Message: err.Error()})
		}
		versions = append(versions, content)
	}

	textMerge := gitTextMerge(mergeLabelsFrom(args))
	merged, hasConflict, err := issue.MergeFile(
		versions[mergeArgBase],
		versions[mergeArgOurs],
		versions[mergeArgTheirs],
		textMerge,
	)
	if err != nil {
		return failedMerge(&Error{Code: CodeInvalid, Path: path, Message: err.Error()})
	}
	if err := tree.ReplaceFileVerbatim(args[mergeArgOurs], bytes.NewReader(merged)); err != nil {
		return failedMerge(&Error{Code: CodeInvalid, Path: args[mergeArgOurs], Message: err.Error()})
	}

	result := Result{Kind: kindMerge, Data: mergeDriverData{Path: path}}
	if hasConflict {
		result.Failure = &Error{Code: CodeConflict, Path: path, Message: conflictMessage}
	}
	return result
}

// failedMerge is the result of a merge-driver run that wrote nothing.
func failedMerge(failure *Error) Result {
	return Result{Kind: kindMerge, Failure: failure}
}

// mergeLabelsFrom returns the conflict marker labels args carry, falling back to the path with
// (base), (ours) or (theirs) for each label that is absent or arrives unexpanded.
func mergeLabelsFrom(args []string) gitx.MergeLabels {
	path := args[mergeArgPath]
	return gitx.MergeLabels{
		Base:   mergeLabel(args, mergeArgBaseLabel, path+fallbackBaseSuffix),
		Ours:   mergeLabel(args, mergeArgOursLabel, path+fallbackOursSuffix),
		Theirs: mergeLabel(args, mergeArgTheirsLabel, path+fallbackTheirsSuffix),
	}
}

// mergeLabel returns the label at position in args, or fallback when it is absent, empty or an
// unexpanded placeholder.
func mergeLabel(args []string, position int, fallback string) string {
	if position >= len(args) || args[position] == "" || slices.Contains(unexpandedLabels, args[position]) {
		return fallback
	}
	return args[position]
}

// gitTextMerge returns the text merge issue.MergeFile runs on everything outside the Log: it
// writes the three versions to a temp directory, merges them with git merge-file under labels and
// removes the directory again.
func gitTextMerge(labels gitx.MergeLabels) issue.TextMerge {
	return func(base []byte, ours []byte, theirs []byte) (merged []byte, hasConflict bool, err error) {
		dir, err := os.MkdirTemp("", mergeTempDirPattern)
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
			if err := os.WriteFile(path, content, mergeInputPermissions); err != nil {
				return nil, false, err
			}
			paths = append(paths, path)
		}
		return gitx.MergeFile(paths[mergeArgBase], paths[mergeArgOurs], paths[mergeArgTheirs], labels)
	}
}
