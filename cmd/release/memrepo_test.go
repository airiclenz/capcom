package main

import (
	"context"
	"fmt"
	"slices"
)

// The repo methods a memRepo can be told to fail, as one whose command cannot run.
const (
	methodFetchMain    = "FetchMain"
	methodIsClean      = "IsClean"
	methodResolve      = "Resolve"
	methodParents      = "Parents"
	methodChangedPaths = "ChangedPaths"
	methodIsAncestor   = "IsAncestor"
	methodHasGreenRun  = "HasGreenRun"
)

// memRepo is the in-memory repo: the checkout and its GitHub repository held as plain state —
// a commit history, where HEAD and origin/main stand, whether the tree is dirty, and which
// commits ci.yml ran green for — so a test states what the repo holds instead of what git and
// gh print.
type memRepo struct {
	commits    map[string]fakeCommit
	head       string
	originMain string
	isDirty    bool
	// greenRuns are the commits ci.yml has a successful run for.
	greenRuns []string
	// failing names a method — methodParents — that fails outright, as one that cannot run.
	failing string
}

// failure returns the error of a method the repo was told to fail, nil for any other.
func (r memRepo) failure(method string) error {
	if r.failing == method {
		return fmt.Errorf("%s: cannot run", method)
	}
	return nil
}

// FetchMain fetches nothing: the state already holds origin/main.
func (r memRepo) FetchMain(context.Context) error {
	return r.failure(methodFetchMain)
}

// IsClean reports the opposite of isDirty.
func (r memRepo) IsClean(context.Context) (bool, error) {
	if err := r.failure(methodIsClean); err != nil {
		return false, err
	}
	return !r.isDirty, nil
}

// Resolve turns HEAD and origin/main into the commits they stand at, and accepts any commit of
// the history as itself.
func (r memRepo) Resolve(_ context.Context, revision string) (string, error) {
	if err := r.failure(methodResolve); err != nil {
		return "", err
	}
	commit := r.resolve(revision)
	if _, isKnown := r.commits[commit]; !isKnown {
		return "", fmt.Errorf("unknown revision %s", revision)
	}
	return commit, nil
}

// Parents returns the commit's parents from the history.
func (r memRepo) Parents(_ context.Context, commit string) ([]string, error) {
	if err := r.failure(methodParents); err != nil {
		return nil, err
	}
	known, isKnown := r.commits[commit]
	if !isKnown {
		return nil, fmt.Errorf("unknown commit %s", commit)
	}
	return slices.Clone(known.parents), nil
}

// ChangedPaths returns the paths the commit changes. The history records them against the only
// parent of an ordinary commit, so asking about a merge or against another commit is refused.
func (r memRepo) ChangedPaths(_ context.Context, parent, commit string) ([]string, error) {
	if err := r.failure(methodChangedPaths); err != nil {
		return nil, err
	}
	known := r.commits[commit]
	if len(known.parents) != 1 || known.parents[0] != parent {
		return nil, fmt.Errorf("changed paths asked for %s against %s, not its only parent", commit, parent)
	}
	return slices.Clone(known.changed), nil
}

// IsAncestor walks the history from descendant.
func (r memRepo) IsAncestor(_ context.Context, ancestor, descendant string) (bool, error) {
	if err := r.failure(methodIsAncestor); err != nil {
		return false, err
	}
	return r.reaches(r.resolve(descendant), r.resolve(ancestor)), nil
}

// HasGreenRun reports whether the commit is among greenRuns.
func (r memRepo) HasGreenRun(_ context.Context, commit string) (bool, error) {
	if err := r.failure(methodHasGreenRun); err != nil {
		return false, err
	}
	return slices.Contains(r.greenRuns, commit), nil
}

// resolve turns the symbolic names into commits.
func (r memRepo) resolve(revision string) string {
	switch revision {
	case "HEAD":
		return r.head
	case remoteMain:
		return r.originMain
	}
	return revision
}

// reaches reports whether target is from or one of its ancestors.
func (r memRepo) reaches(from, target string) bool {
	if from == target {
		return true
	}
	return slices.ContainsFunc(r.commits[from].parents, func(parent string) bool {
		return r.reaches(parent, target)
	})
}
