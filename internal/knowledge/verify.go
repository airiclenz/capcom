package knowledge

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// symbolSeparator splits a Ref into its path and the symbol text the file must contain.
const symbolSeparator = "#"

// parentPrefix starts a relative path that climbs out of the directory it is relative to.
const parentPrefix = ".."

// ErrEscapes reports a Ref that names a path outside the working tree.
var ErrEscapes = errors.New("escapes the working tree")

// errNoPath reports a Ref without a path before its symbol.
var errNoPath = errors.New("names no path; write path or path" + symbolSeparator + "symbol")

// Finding is one Ref of a Note that does not resolve in the working tree.
type Finding struct {
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

// Verify checks every Ref of note, the Note with id noteID stored at notePath (relative to
// worktreeRoot with forward slashes), and returns a Finding for each one that does not resolve: a
// malformed Ref, one escaping the working tree (symbolic links included), a missing path (one
// below a file included), a `path#symbol` whose file does not contain the symbol text, and a
// Ref the disk cannot resolve (a symbolic-link loop, a name too long, a permission refused). A Note
// without Refs has no Findings. It fails only when worktreeRoot itself cannot be resolved.
func Verify(worktreeRoot string, notePath string, noteID string, note Note) ([]Finding, error) {
	root, err := filepath.EvalSymlinks(worktreeRoot)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", worktreeRoot, err)
	}
	var findings []Finding
	for i, ref := range note.Refs {
		message := checkRef(root, ref)
		if message == "" {
			continue
		}
		findings = append(findings, Finding{
			NoteID:  noteID,
			Path:    notePath,
			Line:    note.RefLine(i),
			Ref:     ref,
			Message: message,
		})
	}
	return findings, nil
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

// checkRef resolves one Ref below root and returns why it does not resolve, or "" when it does.
// Every reason is a message, the disk's own errors included: one unreadable Ref never stops the
// check of the others.
func checkRef(root string, ref string) string {
	refPath, symbol, formErr := splitRef(ref)
	if formErr != nil {
		return formErr.Error()
	}
	target := filepath.Join(root, filepath.FromSlash(refPath))
	resolved, err := filepath.EvalSymlinks(target)
	if isMissing(err) {
		return "no such file or directory"
	}
	if err != nil {
		return unresolvable(err)
	}
	if relative, relErr := filepath.Rel(root, resolved); relErr != nil || isOutside(relative) {
		return ErrEscapes.Error() + " through a symbolic link"
	}
	if symbol == "" {
		return ""
	}
	return checkSymbol(resolved, symbol)
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

// checkSymbol returns why the file at path does not hold symbol, or "" when it does.
func checkSymbol(path string, symbol string) string {
	info, err := os.Stat(path)
	if err != nil {
		return unresolvable(err)
	}
	if info.IsDir() {
		return fmt.Sprintf("is a directory; symbol %q needs a file", symbol)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return unresolvable(err)
	}
	if !strings.Contains(string(data), symbol) {
		return fmt.Sprintf("symbol %q is not in the file", symbol)
	}
	return ""
}
