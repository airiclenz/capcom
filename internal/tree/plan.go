package tree

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
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
	rename func(oldPath string, newPath string) error
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
// added. Taking a rewrite back writes Original byte for byte, CRLF included, with the permissions
// the file had just before Apply overwrote it.
func (p *Plan) AddRewrite(rewrite Rewrite) {
	var mode fs.FileMode
	p.rewrites = append(p.rewrites, step{
		apply: func() error {
			info, err := os.Stat(rewrite.Path)
			if err != nil {
				return err
			}
			mode = info.Mode()
			return WriteFileAtomic(rewrite.Path, bytes.NewReader(rewrite.Content))
		},
		undo: func() error {
			return WriteFileAtomicVerbatim(rewrite.Path, bytes.NewReader(rewrite.Original), mode)
		},
	})
}

// AddCleanup adds a best-effort change Apply makes once every step has landed. A cleanup is never
// taken back and its failure fails nothing: Apply returns it as a warning.
func (p *Plan) AddCleanup(cleanup func() error) {
	p.cleanups = append(p.cleanups, cleanup)
}

// Apply carries out the plan: the rewrites, then the other steps in the order added. When any of
// them fails it takes back the ones already made, newest first, so the files are as they were, and
// returns an error that names the failing path and any change that could not be taken back. Once
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

// renameStep renames oldPath to newPath, refusing to replace anything already at newPath, and
// renames it back to undo it.
func (p *Plan) renameStep(oldPath string, newPath string) step {
	return step{
		apply: func() error {
			if _, err := os.Lstat(newPath); !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("%s: %w", newPath, ErrMoveTarget)
			}
			return p.doRename(oldPath, newPath)
		},
		undo: func() error { return p.doRename(newPath, oldPath) },
	}
}

// doRename renames oldPath to newPath with the plan's rename, os.Rename unless a test swapped it.
func (p *Plan) doRename(oldPath string, newPath string) error {
	if p.rename == nil {
		return os.Rename(oldPath, newPath)
	}
	return p.rename(oldPath, newPath)
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
