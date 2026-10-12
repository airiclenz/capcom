package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// The repo methods a memRepo can be told to fail, as one whose command cannot run.
const (
	methodFetchMain     = "FetchMain"
	methodIsClean       = "IsClean"
	methodResolve       = "Resolve"
	methodParents       = "Parents"
	methodChangedPaths  = "ChangedPaths"
	methodIsAncestor    = "IsAncestor"
	methodHasGreenRun   = "HasGreenRun"
	methodReleaseTags   = "ReleaseTags"
	methodCommitFile    = "CommitFile"
	methodCountCommits  = "CountCommits"
	methodLatestRelease = "LatestRelease"
	methodRemoteHasTag  = "RemoteHasTag"
	methodRemoteTagType = "RemoteTagType"
	methodReleaseAssets = "ReleaseAssets"
)

// memRepo is the in-memory repo: the checkout and its GitHub repository held as plain state —
// a commit history, where HEAD and origin/main stand, whether the tree is dirty, the tags, which
// commits ci.yml ran green for, origin's tags and the GitHub releases — so a test states what the
// repo holds instead of what git and gh print.
type memRepo struct {
	commits    map[string]fakeCommit
	head       string
	originMain string
	isDirty    bool
	// greenRuns are the commits ci.yml has a successful run for.
	greenRuns []string
	// tags are the checkout's tags, release tags or not.
	tags []string
	// madeCommits collects every commit CommitFile makes, in order; a memRepo without one refuses
	// CommitFile. It is a pointer so a memRepo handed out by value still records into the
	// test's log.
	madeCommits *[]memCommit
	// commitCounts is how many commits each revision range holds; a range it lacks is unknown.
	commitCounts map[string]int
	// latestRelease is the latest published GitHub release, its zero value while none is.
	latestRelease githubRelease
	// remoteTags are origin's tags, each with the type of the object GitHub's ref points at:
	// "tag" for an annotated tag, "commit" for a lightweight one.
	remoteTags map[string]string
	// releases are the GitHub releases by tag, each with its fully uploaded assets.
	releases map[string][]string
	// failing names a method — methodParents — that fails outright, as one that cannot run.
	failing string
}

// memCommit is one commit memRepo.CommitFile made: its subject and the one file it wrote.
type memCommit struct {
	subject string
	path    string
	content string
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

// ReleaseTags returns the tags whose name starts with v.
func (r memRepo) ReleaseTags(context.Context) ([]string, error) {
	if err := r.failure(methodReleaseTags); err != nil {
		return nil, err
	}
	releases := []string{}
	for _, tag := range r.tags {
		if strings.HasPrefix(tag, "v") {
			releases = append(releases, tag)
		}
	}
	return releases, nil
}

// CommitFile records the commit in madeCommits.
func (r memRepo) CommitFile(_ context.Context, path string, content []byte, subject string) error {
	if err := r.failure(methodCommitFile); err != nil {
		return err
	}
	if r.madeCommits == nil {
		return errors.New("this memRepo records no commits")
	}
	*r.madeCommits = append(*r.madeCommits, memCommit{subject: subject, path: path, content: string(content)})
	return nil
}

// CountCommits returns the range's count from commitCounts.
func (r memRepo) CountCommits(_ context.Context, revisionRange string) (int, error) {
	if err := r.failure(methodCountCommits); err != nil {
		return 0, err
	}
	count, isKnown := r.commitCounts[revisionRange]
	if !isKnown {
		return 0, fmt.Errorf("unknown revision range %s", revisionRange)
	}
	return count, nil
}

// LatestRelease returns latestRelease, and false while its tag is empty.
func (r memRepo) LatestRelease(context.Context) (githubRelease, bool, error) {
	if err := r.failure(methodLatestRelease); err != nil {
		return githubRelease{}, false, err
	}
	return r.latestRelease, r.latestRelease.Tag != "", nil
}

// RemoteHasTag reports whether the tag is among remoteTags.
func (r memRepo) RemoteHasTag(_ context.Context, tag string) (bool, error) {
	if err := r.failure(methodRemoteHasTag); err != nil {
		return false, err
	}
	_, hasTag := r.remoteTags[tag]
	return hasTag, nil
}

// RemoteTagType returns the tag's object type from remoteTags.
func (r memRepo) RemoteTagType(_ context.Context, tag string) (string, error) {
	if err := r.failure(methodRemoteTagType); err != nil {
		return "", err
	}
	kind, hasTag := r.remoteTags[tag]
	if !hasTag {
		return "", fmt.Errorf("origin has no tag %s", tag)
	}
	return kind, nil
}

// ReleaseAssets returns the release's assets from releases.
func (r memRepo) ReleaseAssets(_ context.Context, tag string) ([]string, bool, error) {
	if err := r.failure(methodReleaseAssets); err != nil {
		return nil, false, err
	}
	assets, isFound := r.releases[tag]
	return slices.Clone(assets), isFound, nil
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
