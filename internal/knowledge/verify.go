package knowledge

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/airiclenz/capcom/internal/diagnose"
	"github.com/airiclenz/capcom/internal/nodefile"
	"github.com/airiclenz/capcom/internal/tree"
)

// symbolSeparator splits a Ref into its path and the symbol text the file must contain.
const symbolSeparator = "#"

// parentPrefix starts a relative path that climbs out of the directory it is relative to.
const parentPrefix = ".."

// gitDirName is the name of the git directory, or of the file pointing to it in a linked worktree,
// at the root of a working tree.
const gitDirName = ".git"

// Why a Ref resolves here yet a clone of the repository lacks it.
const (
	inGitDirMessage  = "is inside the git directory, which a clone does not share"
	isIgnoredMessage = "is ignored by git, so a clone does not have it"
)

// ErrEscapes reports a Ref that names a path outside the working tree.
var ErrEscapes = errors.New("escapes the working tree")

// inGitDir is the failed check of a Ref inside the git directory.
var inGitDir = refCheck{code: diagnose.UnsharedRef, message: inGitDirMessage, isInGitDir: true}

// errNoPath reports a Ref without a path before its symbol.
var errNoPath = errors.New("names no path; write path or path" + symbolSeparator + "symbol")

// Finding is one Ref of a Note that does not resolve in the working tree, or that resolves here
// but names a path a clone of the repository lacks.
type Finding struct {
	// Code is diagnose.BrokenRef for a Ref that does not resolve, or diagnose.UnsharedRef for one
	// into the git directory or to a path git ignores.
	Code diagnose.Kind
	// IsInGitDir is set on an UnsharedRef Finding whose Ref is inside the git directory, and unset
	// on one git ignores.
	IsInGitDir bool
	// NoteID is the id of the Note holding the Ref.
	NoteID string
	// Path is the Note file, relative to the working tree root with forward slashes.
	Path string
	// Line is the 1-based line of the Ref in the Note file, or 0 when unknown.
	Line int
	// Ref is the Ref as the Note writes it.
	Ref string
	// Message says why the Ref does not resolve.
	Message string
}

// String renders "path:line: ref "<ref>": message", or "path: ..." when the line is unknown.
func (f Finding) String() string {
	location := f.Path
	if f.Line > 0 {
		location = fmt.Sprintf("%s:%d", f.Path, f.Line)
	}
	return fmt.Sprintf("%s: ref %q: %s", location, f.Ref, f.Message)
}

// ValidateRef checks the form of a Ref: a non-empty path relative to the working tree root that
// stays inside it, optionally followed by `#` and a non-empty symbol. It does not look at the
// disk. The error wraps ErrEscapes for a path that climbs out of the tree or is absolute.
func ValidateRef(ref string) error {
	if _, _, err := splitRef(ref); err != nil {
		return fmt.Errorf("ref %q %w", ref, err)
	}
	return nil
}

// RewriteRefs returns refs with every Ref that names a moved node file or its children directory
// rewritten to where it lives after the move, and reports whether any Ref was. oldFile and newFile
// are the node file before and after the move, oldDir and newDir its children directory, all
// relative to the working tree root with forward slashes. A Ref whose path, cleaned, equals oldFile
// becomes newFile, one equal to oldDir becomes newDir, and one below oldDir keeps the rest of its
// path below newDir; its `#symbol` is kept. Every other Ref, a malformed one included, is kept as it
// is. refs is returned unchanged, not copied, when no Ref is rewritten.
func RewriteRefs(refs []string, oldFile string, newFile string, oldDir string, newDir string) ([]string, bool) {
	var rewritten []string
	for i, ref := range refs {
		moved, isMoved := movedRef(ref, oldFile, newFile, oldDir, newDir)
		if !isMoved {
			continue
		}
		if rewritten == nil {
			rewritten = slices.Clone(refs)
		}
		rewritten[i] = moved
	}
	if rewritten == nil {
		return refs, false
	}
	return rewritten, true
}

// movedRef returns ref rewritten as RewriteRefs rewrites it, and whether it names the moved file or
// directory at all.
func movedRef(ref string, oldFile string, newFile string, oldDir string, newDir string) (string, bool) {
	refPath, symbol, err := splitRef(ref)
	if err != nil {
		return ref, false
	}
	cleaned := path.Clean(refPath)
	var moved string
	switch {
	case cleaned == oldFile:
		moved = newFile
	case cleaned == oldDir:
		moved = newDir
	case strings.HasPrefix(cleaned, oldDir+"/"):
		moved = newDir + strings.TrimPrefix(cleaned, oldDir)
	default:
		return ref, false
	}
	if symbol != "" {
		moved += symbolSeparator + symbol
	}
	return moved, true
}

// IgnoreFunc returns the subset of paths, relative to the working tree root dir with forward
// slashes, that git ignores; gitx.IgnoredPaths is one.
type IgnoreFunc func(dir string, paths []string) ([]string, error)

// Repository is the git working tree Verify checks Refs against.
type Repository struct {
	// WorktreeRoot is the top directory of the working tree.
	WorktreeRoot string
	// GitDir is this worktree's git directory, or "" when there is none to check against.
	GitDir string
	// GitCommonDir is the git directory every worktree shares, or "" when there is none.
	GitCommonDir string
	// IgnoredPaths says which paths git ignores; nil treats no path as ignored.
	IgnoredPaths IgnoreFunc
}

// NoteFile is one Note for Verify: the Note, its id, and its file relative to the working tree
// root with forward slashes.
type NoteFile struct {
	ID   string
	Path string
	Note Note
}

// refCheck is the state of one Ref while Verify checks it: a finding's Code and Message once it
// fails, or the resolved path and symbol still waiting for the ignore and symbol checks.
type refCheck struct {
	code       diagnose.Kind
	message    string
	isInGitDir bool
	resolved   string
	symbol     string
}

// Verify checks every Ref of notes and returns a Finding for each one that does not resolve
// (Code diagnose.BrokenRef): a malformed Ref, one escaping the working tree (symbolic links
// included), a missing path (one below a file included), a `path#symbol` whose file does not
// contain the symbol text or is not a regular file of at most 1 MiB, and a Ref the disk cannot
// resolve (a symbolic-link loop, a name too long, a permission refused). It also returns one for
// each Ref that resolves here but that a clone lacks (Code diagnose.UnsharedRef): one whose first
// path component is .git, one resolving inside the git directory or the git common directory, and
// one whose resolved path git ignores. Findings follow the order of notes and of their Refs; a
// Note without Refs has none. It asks repository.IgnoredPaths once per call, and fails only when
// that fails or when the working tree root itself cannot be resolved.
func Verify(repository Repository, notes []NoteFile) ([]Finding, error) {
	root, err := filepath.EvalSymlinks(repository.WorktreeRoot)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", repository.WorktreeRoot, err)
	}
	gitDirs := resolvedGitDirs(root, repository.GitDir, repository.GitCommonDir)
	checks := make([][]refCheck, len(notes))
	var pending []string
	for i, file := range notes {
		checks[i] = make([]refCheck, len(file.Note.Refs))
		for j, ref := range file.Note.Refs {
			checks[i][j] = checkRef(root, gitDirs, ref)
			if checks[i][j].resolved != "" {
				pending = append(pending, checks[i][j].resolved)
			}
		}
	}

	ignored, err := ignoredSet(repository.IgnoredPaths, root, pending)
	if err != nil {
		return nil, err
	}
	var findings []Finding
	for i, file := range notes {
		for j, ref := range file.Note.Refs {
			check := finishCheck(root, checks[i][j], ignored)
			if check.message == "" {
				continue
			}
			findings = append(findings, Finding{
				Code:       check.code,
				IsInGitDir: check.isInGitDir,
				NoteID:     file.ID,
				Path:       file.Path,
				Line:       file.Note.RefLine(j),
				Ref:        ref,
				Message:    check.message,
			})
		}
	}
	return findings, nil
}

// resolvedGitDirs returns the non-empty git directories with their symbolic links resolved, so a
// resolved Ref compares against them; a directory that cannot be resolved is kept as given. A git
// directory holding the working tree root is left out, or it would claim every Ref.
func resolvedGitDirs(root string, dirs ...string) []string {
	var resolved []string
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		if resolvedDir, err := filepath.EvalSymlinks(dir); err == nil {
			dir = resolvedDir
		}
		dir = filepath.Clean(dir)
		if !isWithin(dir, root) {
			resolved = append(resolved, dir)
		}
	}
	return resolved
}

// ignoredSet asks isIgnored once which of paths, relative to root, git ignores, and returns them
// as a set. A nil isIgnored, or no paths, ignores none.
func ignoredSet(isIgnored IgnoreFunc, root string, paths []string) (map[string]bool, error) {
	set := map[string]bool{}
	if isIgnored == nil || len(paths) == 0 {
		return set, nil
	}
	ignored, err := isIgnored(root, paths)
	if err != nil {
		return nil, fmt.Errorf("find the git-ignored Refs: %w", err)
	}
	for _, path := range ignored {
		set[path] = true
	}
	return set, nil
}

// splitRef splits a Ref into its slash path and symbol ("" when it names none). Its error, which
// checks the form only, reads after the Ref: `ref "x" <error>`.
func splitRef(ref string) (refPath string, symbol string, err error) {
	refPath, symbol, hasSymbol := strings.Cut(ref, symbolSeparator)
	nativePath := filepath.FromSlash(refPath)
	switch {
	case refPath == "":
		return "", "", errNoPath
	case hasSymbol && symbol == "":
		return "", "", errors.New("has an empty symbol after " + symbolSeparator)
	case strings.HasPrefix(refPath, "/") || filepath.IsAbs(nativePath) || filepath.VolumeName(nativePath) != "":
		return "", "", fmt.Errorf("%w: write a path relative to its root", ErrEscapes)
	case isOutside(filepath.Clean(nativePath)):
		return "", "", ErrEscapes
	}
	return refPath, symbol, nil
}

// isOutside reports whether a cleaned relative path climbs out of the directory it is relative to.
func isOutside(relative string) bool {
	return relative == parentPrefix || strings.HasPrefix(relative, parentPrefix+string(filepath.Separator))
}

// checkRef resolves one Ref below root and returns its failure, or, for a Ref resolving inside
// the tree outside gitDirs, its resolved path relative to root, which finishCheck completes.
// Every reason is a message, the disk's own errors included: one unreadable Ref never stops the
// check of the others.
func checkRef(root string, gitDirs []string, ref string) refCheck {
	refPath, symbol, formErr := splitRef(ref)
	if formErr != nil {
		return broken(formErr.Error())
	}
	if isGitDirName(refPath) {
		return inGitDir
	}
	target := filepath.Join(root, filepath.FromSlash(refPath))
	resolved, err := filepath.EvalSymlinks(target)
	if isMissing(err) {
		return broken("no such file or directory")
	}
	if err != nil {
		return broken(unresolvable(err))
	}
	for _, gitDir := range gitDirs {
		if isWithin(gitDir, resolved) {
			return inGitDir
		}
	}
	relative, relErr := filepath.Rel(root, resolved)
	if relErr != nil || isOutside(relative) {
		return broken(ErrEscapes.Error() + " through a symbolic link")
	}
	return refCheck{resolved: filepath.ToSlash(relative), symbol: symbol}
}

// finishCheck completes a Ref checkRef resolved inside the tree: a path git ignores is unshared,
// and otherwise its symbol, if any, must be in the file. A failed check is returned as it is.
func finishCheck(root string, check refCheck, ignored map[string]bool) refCheck {
	switch {
	case check.resolved == "":
		return check
	case ignored[check.resolved]:
		return refCheck{code: diagnose.UnsharedRef, message: isIgnoredMessage}
	case check.symbol == "":
		return refCheck{}
	}
	return broken(checkSymbol(filepath.Join(root, filepath.FromSlash(check.resolved)), check.symbol))
}

// broken is the failed check of a Ref that does not resolve, or the passed check when message is
// "".
func broken(message string) refCheck {
	if message == "" {
		return refCheck{}
	}
	return refCheck{code: diagnose.BrokenRef, message: message}
}

// isGitDirName reports whether the first component of a Ref's cleaned slash path is .git.
func isGitDirName(refPath string) bool {
	first, _, _ := strings.Cut(filepath.ToSlash(filepath.Clean(filepath.FromSlash(refPath))), "/")
	return first == gitDirName
}

// isWithin reports whether path is dir or lies below it; both are absolute and cleaned.
func isWithin(dir string, path string) bool {
	relative, err := filepath.Rel(dir, path)
	return err == nil && !isOutside(relative) && !filepath.IsAbs(relative)
}

// unresolvable renders a disk error on a Ref as "cannot be resolved: <reason>". The reason drops
// the absolute path a path error carries, since the Finding already names the Ref.
func unresolvable(err error) string {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		err = pathErr.Err
	}
	return "cannot be resolved: " + err.Error()
}

// isMissing reports whether err says a path does not exist. A path below a file (`file.go/sub`)
// fails with ENOTDIR on Unix rather than ENOENT, yet it is just as absent.
func isMissing(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}

// checkSymbol returns why the file at path does not hold symbol, or "" when it does. It reads the
// file only through the guarded reader, so a named pipe or other non-regular entry, or a file
// larger than 1 MiB, is a reason rather than a hang or a large read.
func checkSymbol(path string, symbol string) string {
	info, err := os.Stat(path)
	if err != nil {
		return unresolvable(err)
	}
	if info.IsDir() {
		return fmt.Sprintf("is a directory; symbol %q needs a file", symbol)
	}
	data, err := nodefile.ReadRegularFile(path)
	switch {
	case errors.Is(err, tree.ErrNotRegular):
		return fmt.Sprintf("is not a regular file; symbol %q needs a file", symbol)
	case errors.Is(err, tree.ErrTooLarge):
		return fmt.Sprintf("is larger than 1 MiB; symbol %q is not searched", symbol)
	case err != nil:
		return unresolvable(err)
	}
	if !bytes.Contains(data, []byte(symbol)) {
		return fmt.Sprintf("symbol %q is not in the file", symbol)
	}
	return ""
}
