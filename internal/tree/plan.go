package tree

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"

	"github.com/airiclenz/capcom/internal/nodefile"
)

// rewriteOp is the Op of the *fs.PathError a rewrite fails with when its file changed.
const rewriteOp = "rewrite"

// crlf and lf are the line endings nodefile.ReplaceFileAtomic converts from and to.
const (
	crlf = "\r\n"
	lf   = "\n"
)

// Plan is a set of changes on disk worked out before any of them is made, then carried out by
// Apply as one: every step either lands or is taken back. Apply runs the rewrites (AddRewrite)
// first, then the other steps (AddStep) in the order added, and once all of them have landed the
// cleanups (AddCleanup), which are best-effort. The zero Plan is empty and ready to use.
type Plan struct {
	rewrites []step
	steps    []step
	cleanups []func() error
	// rename renames a file or directory, os.Rename when nil; tests swap it to inject a failure.
	// While it is swapped, a node file is renamed with it too, instead of hard-linked.
	rename func(oldPath string, newPath string) error
	// remove removes a file or an empty directory, os.Remove when nil; tests swap it to inject a
	// failure.
	remove func(path string) error
}

// step is one change a Plan makes on disk and the change that takes it back.
type step struct {
	apply func() error
	undo  func() error
}

// AddStep adds a change apply that undo takes back. Apply makes the steps in the order added, after
// every rewrite, and undoes the ones already made, newest first, when a later step fails.
func (p *Plan) AddStep(apply func() error, undo func() error) {
	p.steps = append(p.steps, step{apply: apply, undo: undo})
}

// AddRewrite adds a file whose content the plan replaces. Apply writes every rewrite atomically at
// the file's current path, in the order added, before any other step, however late the rewrite was
// added, and only over a file that still holds Original byte for byte: a file read again as other
// bytes fails Apply with a *fs.PathError naming it that wraps ErrChanged, before anything is
// written to it. The write never recreates a file or directory: a file gone just before it fails
// Apply with an error naming it that wraps fs.ErrNotExist. Taking a rewrite back writes Original byte for byte, CRLF included, with the
// permissions the file had just before Apply overwrote it, and only over a file that still holds
// the bytes the rewrite wrote; a file changed since is left as it is and named as a failure to
// take the rewrite back, which does not wrap ErrChanged.
func (p *Plan) AddRewrite(rewrite Rewrite) {
	var mode fs.FileMode
	// nodefile.ReplaceFileAtomic turns every CRLF of Content into LF, so these are the bytes it
	// leaves.
	written := bytes.ReplaceAll(rewrite.Content, []byte(crlf), []byte(lf))
	p.rewrites = append(p.rewrites, step{
		apply: func() error {
			info, err := os.Stat(rewrite.Path)
			if err != nil {
				return err
			}
			mode = info.Mode()
			if err := checkUnchanged(rewrite.Path, rewrite.Original); err != nil {
				return err
			}
			return nodefile.ReplaceFileAtomic(rewrite.Path, bytes.NewReader(rewrite.Content))
		},
		undo: func() error {
			if err := checkUnchanged(rewrite.Path, written); err != nil {
				if errors.Is(err, ErrChanged) {
					return fmt.Errorf("%s: changed since the rewrite; left as it is", rewrite.Path)
				}
				return err
			}
			return nodefile.WriteFileAtomicVerbatim(rewrite.Path, bytes.NewReader(rewrite.Original), mode)
		},
	})
}

// checkUnchanged reads the file at path through nodefile.ReadRegularFile and fails with a
// *fs.PathError naming path that wraps ErrChanged when it does not hold expected byte for byte. A
// failed read is returned as it is.
func checkUnchanged(path string, expected []byte) error {
	current, err := nodefile.ReadRegularFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, expected) {
		return &fs.PathError{Op: rewriteOp, Path: path, Err: ErrChanged}
	}
	return nil
}

// AddCleanup adds a best-effort change Apply makes once every step has landed. A cleanup is never
// taken back and its failure fails nothing: Apply returns it as a warning.
func (p *Plan) AddCleanup(cleanup func() error) {
	p.cleanups = append(p.cleanups, cleanup)
}

// Apply carries out the plan: the rewrites, then the other steps in the order added. A rewrite
// lands only on a file that still holds its Original, and fails with ErrChanged otherwise. When any
// of them fails it takes back the ones already made, newest first, so the files are as they were,
// and returns an error that names the failing path and any change that could not be taken back,
// a rewritten file changed since its rewrite among them, left as it is. Once
// every step has landed it runs the cleanups in the order added and returns their failures as
// warnings, in that order. Apply must run at most once.
func (p *Plan) Apply() (warnings []error, err error) {
	steps := slices.Concat(p.rewrites, p.steps)
	for applied, step := range steps {
		if err := step.apply(); err != nil {
			return nil, rollBack(steps[:applied], err)
		}
	}
	for _, cleanup := range p.cleanups {
		if err := cleanup(); err != nil {
			warnings = append(warnings, err)
		}
	}
	return warnings, nil
}

// removeFile removes a node file the file rename step leaves behind; a test swaps it to make the
// removal fail.
var removeFile = os.Remove

// renameStep renames the directory oldPath to newPath, refusing to replace anything already at
// newPath, and renames it back to undo it.
func (p *Plan) renameStep(oldPath string, newPath string) step {
	return step{
		apply: func() error { return p.checkedRename(oldPath, newPath) },
		undo:  func() error { return p.doRename(newPath, oldPath) },
	}
}

// fileRenameStep moves the node file oldPath to newPath without replacing a file there, however
// late that file appeared, and moves it back the same way to undo it. See moveFile.
func (p *Plan) fileRenameStep(oldPath string, newPath string) step {
	return step{
		apply: func() error { return p.moveFile(oldPath, newPath) },
		undo:  func() error { return p.moveFile(newPath, oldPath) },
	}
}

// moveFile gives the file oldPath the name newPath with a hard link, which the file system refuses
// atomically when newPath exists, then removes oldPath. An existing newPath fails with an error
// naming it that wraps ErrMoveTarget, and both files stay as they are. When removing oldPath fails
// the link is taken back, so no second name is left, and the error names oldPath. Where the file
// system refuses the link for any other reason, or a test swapped the plan's rename, it falls back
// to checkedRename, which only checks newPath before renaming.
func (p *Plan) moveFile(oldPath string, newPath string) error {
	if p.rename != nil {
		return p.checkedRename(oldPath, newPath)
	}
	err := nodefile.Link(oldPath, newPath)
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%s: %w", newPath, ErrMoveTarget)
	}
	if err != nil {
		// No hard links here (EPERM, ENOTSUP, Windows' ERROR_INVALID_FUNCTION on FAT).
		return p.checkedRename(oldPath, newPath)
	}
	if err := removeFile(oldPath); err != nil {
		return fmt.Errorf("%s: %w", oldPath, joinNonNil(err, removeFile(newPath)))
	}
	return nil
}

// checkedRename renames oldPath to newPath unless something is already at newPath, which fails
// with an error naming it that wraps ErrMoveTarget. A path that appears between the check and the
// rename is replaced.
func (p *Plan) checkedRename(oldPath string, newPath string) error {
	if _, err := os.Lstat(newPath); !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s: %w", newPath, ErrMoveTarget)
	}
	return p.doRename(oldPath, newPath)
}

// doRename renames oldPath to newPath with the plan's rename, os.Rename unless a test swapped it.
func (p *Plan) doRename(oldPath string, newPath string) error {
	if p.rename == nil {
		return os.Rename(oldPath, newPath)
	}
	return p.rename(oldPath, newPath)
}

// doRemove removes path with the plan's remove, os.Remove unless a test swapped it.
func (p *Plan) doRemove(path string) error {
	if p.remove == nil {
		return os.Remove(path)
	}
	return p.remove(path)
}

// rollBack undoes the applied steps, newest first, and returns cause with every failure to undo
// one appended.
func rollBack(applied []step, cause error) error {
	failures := []error{cause}
	for index := len(applied) - 1; index >= 0; index-- {
		if err := applied[index].undo(); err != nil {
			failures = append(failures, fmt.Errorf("rolling back: %w", err))
		}
	}
	return errors.Join(failures...)
}

// joinNonNil returns the first error wrapped, with any later cleanup errors appended to its text.
func joinNonNil(first error, cleanup ...error) error {
	for _, err := range cleanup {
		if err != nil {
			first = fmt.Errorf("%w (cleanup: %v)", first, err)
		}
	}
	return first
}
