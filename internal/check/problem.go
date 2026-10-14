package check

import (
	"errors"
	"fmt"

	"github.com/airiclenz/capcom/internal/diagnose"
	"github.com/airiclenz/capcom/internal/nodefile"
	"github.com/airiclenz/capcom/internal/tree"
)

// Problem is one thing wrong with the store: a diagnose.Finding, which the index reports too.
type Problem = diagnose.Finding

// addFormatTooOld records a format-too-old problem at line, the line of key, a key that needs
// store format needed in a store declaring format declared.
func (c *checker) addFormatTooOld(path string, line int, key string, needed int, declared int) {
	c.add(path, line, diagnose.FormatTooOld, fmt.Sprintf(
		"%s needs store format %d, but the store declares format %d: set format: %d in %s, "+
			"or run a capcom write verb that raises it",
		key,
		needed,
		declared,
		needed,
		c.relative(c.store.ConfigPath()),
	))
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
		// Every problem tree.Load reports wraps a sentinel TreeKind knows; any other is reported
		// as a duplicate.
		code := diagnose.Duplicate
		if kind, isKnown := diagnose.TreeKind(problem); isKnown {
			code = kindCode(kind)
		}
		c.add(problem.Path, line, code, relative.Message())
	}
	return loaded, nil
}

// kindCode is the Problem code of a problem of kind: a repeated alias is a duplicate, and every
// other Kind is its own code.
func kindCode(kind diagnose.Kind) diagnose.Kind {
	if kind == diagnose.RepeatedAlias {
		return diagnose.Duplicate
	}
	return kind
}

// readFile reads the issue or Note file at path through nodefile.ReadRegularFile. A file it refuses
// unread, one no longer a regular file or grown past 1 MiB since the walk, is recorded as a
// not-regular problem, as the walk records one, and isRead is false with no error. Any other
// failed read is an error naming the file.
func (c *checker) readFile(path string) (data []byte, isRead bool, err error) {
	data, err = nodefile.ReadRegularFile(path)
	if refusal := diagnose.ReadRefusal(err); refusal != nil {
		c.add(path, 0, diagnose.NotRegular, refusal.Error())
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("%s: %w", c.relative(path), err)
	}
	return data, true, nil
}
