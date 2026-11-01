package agents

import (
	"bytes"
	"fmt"
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
	// ImportLine is the line that imports the file the guide must reach from the guide's file
	// (`@AGENTS.md`, or `@../AGENTS.md` for a guide in a subdirectory; for AgentsGuide, the block
	// file, such as `@.capcom/AGENTS-capcom.md`). It is set for a guide read or written:
	// GuideReaches through a line, GuideUnreached and GuideImported; and, for AgentsGuide, for a
	// missing AGENTS.md: the line it would need to reach the block file.
	ImportLine string
	// HasMarkers is set by AgentsGuide only: AGENTS.md, which is not the block file, still holds a
	// capcom marker line.
	HasMarkers bool
	// blockFile is the block file AgentsGuide reports AGENTS.md against; it is empty for a guide
	// Guides reports, which must reach AGENTS.md.
	blockFile string
}

// Problem is the message that reports an unreached guide, naming the file it must reach (AGENTS.md,
// or the block file for AgentsGuide) and the import line that would make it reach the block,
// without the guide's path: callers put Path before it. It is empty for any other State.
func (g Guide) Problem() string {
	if g.State != GuideUnreached {
		return ""
	}
	target := FileName
	if g.blockFile != "" {
		target = g.blockFile
	}
	return "does not reach the capcom block in " + target + "; run capcom agents to add " + g.ImportLine
}

// IsMissingAGENTSMd reports an AGENTS.md that AgentsGuide found missing while the block lives in
// another file: no agent guide then reaches the block, and capcom never creates AGENTS.md. It is
// false for every guide Guides reports.
func (g Guide) IsMissingAGENTSMd() bool {
	return g.blockFile != "" && g.State == GuideMissing
}

// blockHome is the file a guide must reach, AGENTS.md or the block file.
type blockHome struct {
	// target is the file's resolved target: a guide resolving to it is the file itself.
	target string
	// rootPath is the file in the working tree, the tree's links resolved: import lines are
	// relative to it.
	rootPath string
}

// Guides reports, for each agent guide in names (paths relative to worktreeRoot, in slash form),
// whether it reaches the Managed block through the AGENTS.md of worktreeRoot, in the order of
// names. A guide reaches the block when it resolves to the same file as AGENTS.md, or when one of
// its lines, white space trimmed, is `@<rel>` or `@./<rel>`, <rel> being the root AGENTS.md
// relative to the directory of the file the guide resolves to, in slash form. ModeWrite appends
// `@<rel>` to each guide that does not reach it, after a blank line and in the file's line ending,
// keeping its permissions; no other mode writes. Guides sharing one resolved file are read, and
// written, once: the first name the import line was appended through is GuideImported, the later
// ones GuideReaches.
//
// Guides returns nil, nil when blockFile (a path relative to worktreeRoot in slash form, FileName
// for AGENTS.md itself) carries no block: when it is missing, links outside the working tree,
// cannot be read or has markers that cannot be told apart, which the block inspection reports. Only
// that gate reads blockFile: the guides reach AGENTS.md whichever file holds the block, and
// AgentsGuide reports whether AGENTS.md reaches a block file other than itself. A missing guide is
// never created, and one that links outside the working tree is neither read nor written. Guides
// fails with an *Error whose Path is the guide's name on a guide that is not a regular file or is
// larger than 1 MiB, and when a guide cannot be resolved, read or written; every error names the
// guide.
func Guides(worktreeRoot, blockFile string, names []string, mode Mode) ([]Guide, error) {
	home, hasBlock := findBlockHome(worktreeRoot, blockFile)
	if !hasBlock {
		return nil, nil
	}

	guides := make([]Guide, 0, len(names))
	foundByTarget := map[string]Guide{}
	for _, name := range names {
		slashName := slashPath(name)
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
		} else if found.State == GuideImported {
			// The import line is already in the shared file: this name reaches the block through it.
			found.State = GuideReaches
		}
		found.Path = slashName
		guides = append(guides, found)
	}
	return guides, nil
}

// findBlockHome reports whether blockFile, of the working tree at worktreeRoot, carries the block
// and resolves the AGENTS.md the guides must reach. Every way blockFile fails to carry one, an
// error included, is reported as no block: the block inspection reports the block file's own
// faults, so the guides stay silent on them.
func findBlockHome(worktreeRoot, blockFile string) (blockHome, bool) {
	if !carriesBlock(worktreeRoot, slashPath(blockFile)) {
		return blockHome{}, false
	}
	home, err := resolveHome(worktreeRoot, FileName)
	if err != nil {
		return blockHome{}, false
	}
	return home, true
}

// carriesBlock reports whether the file name, a path relative to worktreeRoot in slash form, is
// inside the working tree, can be read and holds one block whose markers can be told apart.
func carriesBlock(worktreeRoot, name string) bool {
	file, err := resolveGuide(worktreeRoot, name)
	if err != nil || file.isMissing || file.isOutside {
		return false
	}
	content, err := readGuide(file)
	if err != nil {
		return false
	}
	span, err := findBlock(name, content)
	return err == nil && span.isFound
}

// resolveHome resolves name, a path relative to worktreeRoot in slash form, as the file a guide
// must reach.
func resolveHome(worktreeRoot, name string) (blockHome, error) {
	file, err := resolveGuide(worktreeRoot, name)
	if err != nil {
		return blockHome{}, fmt.Errorf("%s: %w", name, err)
	}
	root, err := filepath.EvalSymlinks(worktreeRoot)
	if err != nil {
		return blockHome{}, err
	}
	return blockHome{target: file.path, rootPath: namedGuidePath(root, name)}, nil
}

// AgentsGuide reports whether the AGENTS.md of worktreeRoot reaches blockFile, the block file (a
// path relative to worktreeRoot in slash form), as Guides reports a guide reaching AGENTS.md: by
// resolving to blockFile, or by a line `@<rel>` or `@./<rel>`, <rel> being blockFile relative to
// the directory of the file AGENTS.md resolves to. ModeWrite appends `@<rel>` to an AGENTS.md that
// does not reach it, as Guides appends to a guide; a missing AGENTS.md is never created, and one
// that links outside the working tree is neither read nor written. A missing AGENTS.md is
// GuideMissing with the ImportLine it would need, and IsMissingAGENTSMd reports it: no agent guide
// reaches the block then. It does not look inside blockFile: callers gate it on the block's Status.
// The Guide's Path is FileName, its Problem names blockFile, and HasMarkers says whether AGENTS.md,
// read, still holds a capcom marker line, which the block in blockFile makes stray.
//
// AgentsGuide returns the zero Guide, its Path empty, when blockFile is AGENTS.md itself: there is
// nothing to reach. It fails as Guides does on a guide, every error naming AGENTS.md, and when
// blockFile cannot be resolved, naming blockFile.
func AgentsGuide(worktreeRoot, blockFile string, mode Mode) (Guide, error) {
	blockFile = slashPath(blockFile)
	if blockFile == FileName {
		return Guide{}, nil
	}
	home, err := resolveHome(worktreeRoot, blockFile)
	if err != nil {
		return Guide{}, err
	}
	agents, err := resolveGuide(worktreeRoot, FileName)
	if err != nil {
		return Guide{}, fmt.Errorf("%s: %w", FileName, err)
	}
	hasMarkers, err := holdsMarkers(agents, home)
	if err != nil {
		return Guide{}, err
	}

	found, err := reachBlock(agents, home, mode)
	if err != nil {
		return Guide{}, err
	}
	if found.State == GuideMissing {
		// AGENTS.md sits at the root, where blockFile, cleaned and in slash form, is already relative.
		found.ImportLine = importPrefix + blockFile
	}
	found.Path = FileName
	found.HasMarkers = hasMarkers
	found.blockFile = blockFile
	return found, nil
}

// holdsMarkers reports whether guide, read, holds a capcom marker line, tangled or not. A guide
// that is missing, links outside the working tree or resolves to the file of home, whose markers
// are the block's own, holds none.
func holdsMarkers(guide guideFile, home blockHome) (bool, error) {
	if guide.isMissing || guide.isOutside || guide.path == home.target {
		return false, nil
	}
	content, err := readGuide(guide)
	if err != nil {
		return false, err
	}
	span, err := findBlock(guide.name, content)
	// Every marker line either opens a span or is one findBlock cannot tell apart.
	return err != nil || span.isFound, nil
}

// reachBlock finds whether guide reaches the file of home and, when it does not and mode is
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
