package agents

import (
	"bytes"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/airiclenz/capcom/internal/nodefile"
)

// importPrefix opens an import line: `@` followed by the path of the file imported.
const importPrefix = "@"

// currentDirectoryPrefix is the `./` an import line may put before a path in the guide's directory.
const currentDirectoryPrefix = "./"

// GuideState is what Guides found for one agent guide.
type GuideState int

const (
	// GuideMissing is a guide that does not exist, or whose link dangles. It is never created.
	GuideMissing GuideState = iota
	// GuideReaches is a guide that resolves to AGENTS.md itself, or holds a line importing it.
	GuideReaches
	// GuideUnreached is a guide that neither resolves to AGENTS.md nor imports it, left as it is.
	GuideUnreached
	// GuideImported is a guide that did not reach the block and to which ModeWrite appended the
	// import line.
	GuideImported
	// GuideOutside is a guide that links outside the working tree. It is never read or written.
	GuideOutside
)

// Guide is one agent guide as Guides found it.
type Guide struct {
	// Path is the guide's path relative to the working tree, in slash form.
	Path string
	// State is what Guides found, or did, for the guide.
	State GuideState
	// ImportLine is the line that imports AGENTS.md from the guide's file (`@AGENTS.md`, or
	// `@../AGENTS.md` for a guide in a subdirectory). It is set for a guide Guides read or wrote:
	// GuideReaches through a line, GuideUnreached and GuideImported.
	ImportLine string
}

// Problem is the message that reports an unreached guide, naming the import line that would make it
// reach the block, without the guide's path: callers put Path before it. It is empty for any other
// State.
func (g Guide) Problem() string {
	if g.State != GuideUnreached {
		return ""
	}
	return "does not reach the capcom block in " + FileName + "; run capcom agents to add " + g.ImportLine
}

// blockHome is the AGENTS.md that holds the block, as the guides must reach it.
type blockHome struct {
	// target is AGENTS.md's resolved target: a guide resolving to it is AGENTS.md.
	target string
	// rootPath is AGENTS.md at the working tree root, the tree's links resolved: import lines are
	// relative to it.
	rootPath string
}

// Guides reports, for each agent guide in names (paths relative to worktreeRoot, in slash form),
// whether it reaches the Managed block in the AGENTS.md of worktreeRoot, in the order of names. A
// guide reaches the block when it resolves to the same file as AGENTS.md, or when one of its lines,
// white space trimmed, is `@<rel>` or `@./<rel>`, <rel> being the root AGENTS.md relative to the
// directory of the file the guide resolves to, in slash form. ModeWrite appends `@<rel>` to each
// guide that does not reach it, after a blank line and in the file's line ending, keeping its
// permissions; no other mode writes. Guides sharing one resolved file are read, and written, once.
//
// Guides returns nil, nil when AGENTS.md carries no block: when it is missing, links outside the
// working tree, cannot be read or has markers that cannot be told apart, which the block inspection
// reports. A missing guide is never created, and one that links outside the working tree is
// neither read nor written. Guides fails with an *Error whose Path is the guide's name on a guide
// that is not a regular file or is larger than 1 MiB, and when a guide cannot be resolved, read or
// written; every error names the guide.
func Guides(worktreeRoot string, names []string, mode Mode) ([]Guide, error) {
	home, hasBlock := findBlockHome(worktreeRoot)
	if !hasBlock {
		return nil, nil
	}

	guides := make([]Guide, 0, len(names))
	foundByTarget := map[string]Guide{}
	for _, name := range names {
		slashName := path.Clean(filepath.ToSlash(name))
		guide, err := resolveGuide(worktreeRoot, slashName)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", slashName, err)
		}
		found, isSeen := foundByTarget[guide.path]
		if !isSeen {
			found, err = reachBlock(guide, home, mode)
			if err != nil {
				return nil, err
			}
			foundByTarget[guide.path] = found
		}
		found.Path = slashName
		guides = append(guides, found)
	}
	return guides, nil
}

// findBlockHome resolves the AGENTS.md of worktreeRoot and reports whether it carries the block.
// Every way AGENTS.md fails to carry one, an error included, is reported as no block: the block
// inspection reports AGENTS.md's own faults, so the guides stay silent on them.
func findBlockHome(worktreeRoot string) (blockHome, bool) {
	agents, err := resolveGuide(worktreeRoot, FileName)
	if err != nil || agents.isMissing || agents.isOutside {
		return blockHome{}, false
	}
	content, err := readGuide(agents)
	if err != nil {
		return blockHome{}, false
	}
	span, err := findBlock(content)
	if err != nil || !span.isFound {
		return blockHome{}, false
	}
	root, err := filepath.EvalSymlinks(worktreeRoot)
	if err != nil {
		return blockHome{}, false
	}
	return blockHome{target: agents.path, rootPath: filepath.Join(root, FileName)}, true
}

// reachBlock finds whether guide reaches the block in home and, when it does not and mode is
// ModeWrite, appends the import line to the file guide resolves to, which must still exist. The
// Guide it returns carries no Path.
func reachBlock(guide guideFile, home blockHome, mode Mode) (Guide, error) {
	switch {
	case guide.isMissing:
		return Guide{State: GuideMissing}, nil
	case guide.isOutside:
		return Guide{State: GuideOutside}, nil
	case guide.path == home.target:
		return Guide{State: GuideReaches}, nil
	}

	relative, err := filepath.Rel(filepath.Dir(guide.path), home.rootPath)
	if err != nil {
		return Guide{}, fmt.Errorf("%s: %w", guide.name, err)
	}
	relative = filepath.ToSlash(relative)
	importLine := importPrefix + relative
	content, err := readGuide(guide)
	if err != nil {
		return Guide{}, err
	}
	if importsFile(content, relative) {
		return Guide{State: GuideReaches, ImportLine: importLine}, nil
	}
	if mode != ModeWrite {
		return Guide{State: GuideUnreached, ImportLine: importLine}, nil
	}

	// An import line appends as the block does to a file without it.
	appended := splice(content, blockSpan{}, importLine)
	if err := nodefile.ReplaceFileVerbatim(guide.path, bytes.NewReader(appended)); err != nil {
		return Guide{}, fmt.Errorf("%s: %w", guide.name, err)
	}
	return Guide{State: GuideImported, ImportLine: importLine}, nil
}

// importsFile reports whether a line of content, white space trimmed, imports relative, written
// either as `@<relative>` or `@./<relative>`.
func importsFile(content []byte, relative string) bool {
	plain := importPrefix + relative
	dotted := importPrefix + currentDirectoryPrefix + relative
	for _, line := range strings.Split(string(content), lineFeed) {
		text := strings.TrimSpace(line)
		if text == plain || text == dotted {
			return true
		}
	}
	return false
}
