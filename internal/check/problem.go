package check

import (
	"errors"
	"fmt"

	"github.com/airiclenz/capcom/internal/nodefile"
	"github.com/airiclenz/capcom/internal/tree"
)

// The closed set of Problem codes.
const (
	// CodeConfig is a config.yaml that cannot be used.
	CodeConfig = "config"
	// CodeInvalidID is a node file whose name is not a valid id.
	CodeInvalidID = "invalid-id"
	// CodeInvalidAlias is an alias outside the allowed characters.
	CodeInvalidAlias = "invalid-alias"
	// CodeDuplicate is an id or alias used twice in one tree.
	CodeDuplicate = "duplicate"
	// CodeOrphanDirectory is a directory without the node file of the same name.
	CodeOrphanDirectory = "orphan-directory"
	// CodeNotRegular is an entry left out unread because it is not a regular file (a symlink, a
	// named pipe, a device) or is a node file larger than 1 MiB.
	CodeNotRegular = "not-regular"
	// CodeFrontmatter is an issue or Note file that does not parse: a malformed frontmatter, an
	// unknown or repeated key, a value of the wrong shape.
	CodeFrontmatter = "frontmatter"
	// CodeConflict is a git conflict marker left in an issue or Note file by a merge.
	CodeConflict = "conflict"
	// CodeField is a parsed issue or Note whose values are not usable: a missing title, an
	// undeclared status or type, a closed issue without closed or reason, and the like.
	CodeField = "field"
	// CodeMissingDep is a dependency that names no issue.
	CodeMissingDep = "missing-dep"
	// CodeMissingRelated is a related link that names no issue.
	CodeMissingRelated = "missing-related"
	// CodeMissingSupersededBy is a superseded-by link that names no Note.
	CodeMissingSupersededBy = "missing-superseded-by"
	// CodeFormatTooOld is a Note key the store's declared format does not hold yet: retired or
	// superseded-by in a store below format 3.
	CodeFormatTooOld = "format-too-old"
	// CodeCycle is a dependency cycle.
	CodeCycle = "cycle"
	// CodeMissingPlan is a plan found in none of the configured plans directories.
	CodeMissingPlan = "missing-plan"
	// CodeBrokenRef is a Note Ref that does not resolve in the working tree.
	CodeBrokenRef = "broken-ref"
	// CodeAgentsBlock is an AGENTS.md whose Managed block is stale or whose markers are tangled,
	// or one that is not a regular file or is larger than 1 MiB.
	CodeAgentsBlock = "agents-block"
)

// Problem is one thing wrong with the store. Path is relative to the working tree root with
// forward slashes; Line is 1-based, or 0 when no single line is to blame.
type Problem struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// String renders "path:line: message", or "path: message" when the line is unknown.
func (p Problem) String() string {
	if p.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", p.Path, p.Line, p.Message)
	}
	return fmt.Sprintf("%s: %s", p.Path, p.Message)
}

// loadTree loads the tree in dir, recording each of its load problems with every file it names
// relative to the working tree root, and returns what could be placed. aliasesLine returns the
// line of the aliases key in the file at a path aliases parsed, or 0; an alias problem cites it.
func (c *checker) loadTree(
	dir string,
	aliases tree.AliasFunc,
	aliasesLine func(path string) int,
) (*tree.Tree, error) {
	loaded, err := tree.Load(dir, aliases)
	var problems tree.Errors
	if !errors.As(err, &problems) {
		return loaded, err
	}
	for _, problem := range problems {
		relative := *problem
		relative.Other = c.relative(problem.Other)
		line := 0
		if problem.Alias != "" {
			line = aliasesLine(problem.Path)
		}
		c.add(problem.Path, line, treeCode(problem.Err), relative.Message())
	}
	return loaded, nil
}

// treeCode is the Problem code of a tree load problem, by the sentinel it wraps.
func treeCode(err error) string {
	switch {
	case errors.Is(err, tree.ErrInvalidID):
		return CodeInvalidID
	case errors.Is(err, tree.ErrInvalidAlias):
		return CodeInvalidAlias
	case errors.Is(err, tree.ErrOrphanDirectory):
		return CodeOrphanDirectory
	case errors.Is(err, tree.ErrNotRegular), errors.Is(err, tree.ErrTooLarge):
		return CodeNotRegular
	default:
		return CodeDuplicate
	}
}

// readFile reads the issue or Note file at path through nodefile.ReadNodeFile. A file it refuses
// unread, one no longer a regular file or grown past 1 MiB since the walk, is recorded as a
// not-regular problem, as the walk records one, and isRead is false with no error. Any other
// failed read is an error naming the file.
func (c *checker) readFile(path string) (data []byte, isRead bool, err error) {
	data, err = nodefile.ReadNodeFile(path)
	for _, refusal := range []error{tree.ErrNotRegular, tree.ErrTooLarge} {
		if errors.Is(err, refusal) {
			c.add(path, 0, CodeNotRegular, refusal.Error())
			return nil, false, nil
		}
	}
	if err != nil {
		return nil, false, fmt.Errorf("%s: %w", c.relative(path), err)
	}
	return data, true, nil
}
