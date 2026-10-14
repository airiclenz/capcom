package cli

import (
	"strings"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/diagnose"
	"github.com/airiclenz/capcom/internal/nodestore"
	"github.com/airiclenz/capcom/internal/tree"
)

// rootParentArg is the --parent value of mv and know mv that moves a node to the top of its tree.
const rootParentArg = "."

// issueMvHelp is the help text of capcom mv.
var issueMvHelp = commandHelp{
	long: "Rename an Issue, move it below another Issue, or both; --parent . moves it to the top of the " +
		"tree. A rename keeps the old id as an Alias, so old citations still resolve, and rewrites every " +
		"deps entry that named the Issue. Its children move with it. capcom renames the files itself and " +
		"never runs git. It prints the Issue's new id, path and etag.",
	example: "  capcom mv refresh-retry token-refresh-retry          # the old id stays an alias\n" +
		"  capcom mv login-timeout-race --parent login-flow     # file it below login-flow\n" +
		"  capcom mv login-timeout-race --parent .              # back to the top of the tree",
}

// noteMvHelp is the help text of capcom know mv.
var noteMvHelp = commandHelp{
	long: "Rename a Note, move it below another Note, or both; --parent . moves it to the top of the " +
		"Knowledge tree. A rename keeps the old id as an Alias, so old citations still resolve, and " +
		"rewrites every superseded-by entry that named the Note. Its children move with it. capcom " +
		"renames the files itself and never runs git. It prints the Note's new id, path and etag.",
	example: "  capcom know mv release-steps cutting-a-release        # the old id stays an alias\n" +
		"  capcom know mv release-tags --parent .                # to the top of the tree\n" +
		"  capcom know mv release-tags --parent cutting-a-release --if-match <etag>   # etag from know show",
}

// mvOptions holds the flags of one capcom mv or know mv.
type mvOptions struct {
	parent       string
	isReparented bool
	ifMatch      string
}

// moveDestination is the nodestore.Destination of mv and know mv: the new id args[1], when given,
// and parent, when isReparented, rootParentArg naming the top of the tree. An empty or
// whitespace-only parent, when isReparented, is a usage error: only rootParentArg moves to the top.
func moveDestination(args []string, parent string, isReparented bool) (nodestore.Destination, error) {
	if isReparented && strings.TrimSpace(parent) == "" {
		return nodestore.Destination{}, output.UsageErrorf("--parent needs an id or %q", rootParentArg)
	}
	destination := nodestore.Destination{Parent: parent, IsReparented: isReparented}
	if len(args) == 2 {
		destination.ID = args[1]
	}
	if parent == rootParentArg {
		destination.Parent = ""
	}
	return destination, nil
}

// movedWarnings renders problems, the tree problems a move returned, as warnings once the move has
// been applied, leaving out every problem the index refresh already warned about. A path inside
// the moved node's children directory is renamed to where that file now lives, in problems and in
// indexWarnings alike, so no warning names a file by a path that no longer exists. indexWarnings
// are renamed copies of the index refresh's warnings, to be rendered in place of them; the
// report itself is left as it is.
func (a *app) movedWarnings(
	moved nodestore.Moved,
	problems tree.Errors,
) (indexWarnings []diagnose.Finding, warnings []string) {
	var warned tree.Errors
	for _, problem := range problems {
		if !a.isIndexWarnedAbout(problem) {
			warned = append(warned, problem)
		}
	}
	oldDir := tree.ChildrenDir(moved.OldPath) + "/"
	newDir := tree.ChildrenDir(moved.Path) + "/"
	indexWarnings = make([]diagnose.Finding, 0, len(a.indexReport.Warnings))
	for _, warning := range a.indexReport.Warnings {
		warning.Path = movedPath(warning.Path, oldDir, newDir)
		indexWarnings = append(indexWarnings, warning)
	}
	for _, problem := range warned {
		problem.Path = movedPath(problem.Path, oldDir, newDir)
		problem.Other = movedPath(problem.Other, oldDir, newDir)
	}
	return indexWarnings, treeProblemWarnings(warned)
}

// movedPath returns path with its leading oldDir replaced by newDir, or path unchanged when it
// does not lie inside oldDir. All three use forward slashes; both directories end in one.
func movedPath(path string, oldDir string, newDir string) string {
	if rest, isInside := strings.CutPrefix(path, oldDir); isInside {
		return newDir + rest
	}
	return path
}
