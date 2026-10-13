package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// The repo methods a memRepo can be told to fail, as one whose command cannot run.
const (
	methodFetchMain          = "FetchMain"
	methodIsClean            = "IsClean"
	methodResolve            = "Resolve"
	methodParents            = "Parents"
	methodChangedPaths       = "ChangedPaths"
	methodIsAncestor         = "IsAncestor"
	methodHasGreenRun        = "HasGreenRun"
	methodReleaseTags        = "ReleaseTags"
	methodCommitFile         = "CommitFile"
	methodCountCommits       = "CountCommits"
	methodLatestRelease      = "LatestRelease"
	methodRemoteHasTag       = "RemoteHasTag"
	methodRemoteTagType      = "RemoteTagType"
	methodReleaseAssets      = "ReleaseAssets"
	methodPushMain           = "PushMain"
	methodFetchTag           = "FetchTag"
	methodIsAnnotated        = "IsAnnotated"
	methodShowFile           = "ShowFile"
	methodLastCommitTouching = "LastCommitTouching"
	methodCloneAt            = "CloneAt"
	methodCreateRelease      = "CreateRelease"
	methodUploadAssets       = "UploadAssets"
	methodUndraftRelease     = "UndraftRelease"
	methodReadAsset          = "ReadAsset"
	methodDownloadAsset      = "DownloadAsset"
)

// repoCallName is the name a memRepo's changes are recorded under in a runner's call record.
const repoCallName = "repo"

// memRepo is the in-memory repo: the checkout and its GitHub repository held as plain state —
// a commit history, where HEAD and origin/main stand, whether the tree is dirty, the tags, which
// commits ci.yml ran green for, origin's tags, the GitHub releases and their assets' content, the
// files at HEAD and what a publish run pushes, fetches, clones and releases — so a test states
// what the repo holds instead of what git and gh print.
type memRepo struct {
	commits    map[string]fakeCommit
	head       string
	originMain string
	isDirty    bool
	// greenRuns are the commits ci.yml has a successful run for.
	greenRuns []string
	// tags are the checkout's tags, release tags or not.
	tags []string
	// tagCommits are the commits the checkout's tags stand at, by tag, for Resolve; a tag it
	// lacks is not in the checkout.
	tagCommits map[string]string
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
	// releases are the GitHub releases by tag, each with its fully uploaded assets; CreateRelease
	// and UploadAssets add to it.
	releases map[string][]string
	// drafts are the releases, by tag, still drafts — ones an interrupted `gh release create`
	// left; UndraftRelease publishes one.
	drafts map[string]bool
	// assetContent is what ReadAsset reads, and DownloadAsset writes, of a release asset, by
	// name, whatever the release.
	assetContent map[string]string
	// files are the checkout's files at HEAD by path, for ShowFile.
	files map[string]string
	// publish is what a publish run changes; nil for a memRepo that refuses PushMain, FetchTag,
	// CloneAt, CreateRelease, UploadAssets and UndraftRelease. It is a pointer, as madeCommits is, so every copy
	// of the memRepo shares it.
	publish *memPublish
	// failing names a method — methodParents — that fails outright, as one that cannot run.
	failing string
	// failures are the errors methods fail with, by method name, so a test can hand a method an
	// error of its own kind — one wrapping exec.ErrNotFound, as a missing gh gives.
	failures map[string]error
}

// memPublish is the part of a memRepo a publish run changes: the commit pushed, the tags
// the CI workflow made after the push, and the tags fetched into the checkout.
type memPublish struct {
	// log is the runner whose ordered call record PushMain, FetchTag and the release step's
	// CloneAt, CreateRelease, UploadAssets, UndraftRelease, ReadAsset and DownloadAsset join, as
	// calls named repoCallName, so a test reads the repo's calls and the commands the runner ran
	// in one order; nil records nothing.
	log *fakeRunner
	// tagAfterPush is the post-PushMain hook, the tag-on-version-bump workflow: asked on each
	// RemoteHasTag poll after the push with how many polls that makes, it returns the tag origin
	// then holds — its kind and commit — and false while origin holds none.
	tagAfterPush func(polls int) (memTag, bool)
	// headAfterGate, when set, is the commit Resolve answers for HEAD after its first answer: a
	// commit made in the checkout after the gate resolved HEAD.
	headAfterGate string

	mu       sync.Mutex
	isPushed bool
	// pushedCommit is the commit PushMain pushed, which origin/main stands at from then on.
	pushedCommit string
	polls        int
	// headResolves counts Resolve's answers for HEAD.
	headResolves int
	// madeTags are the tags the workflow made after the push; fetched are origin's tags
	// fetched into the checkout. Both are by name.
	madeTags map[string]memTag
	fetched  map[string]memTag
}

// memTag is one tag: the type of the object its ref points at — "tag" for an annotated tag,
// "commit" for a lightweight one — and the commit it is on.
type memTag struct {
	name   string
	kind   string
	commit string
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
	return r.failures[method]
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
// the history as itself. HEAD answers publish's headAfterGate after its first answer, when set.
func (r memRepo) Resolve(_ context.Context, revision string) (string, error) {
	if err := r.failure(methodResolve); err != nil {
		return "", err
	}
	commit := r.resolve(revision)
	if revision == "HEAD" {
		commit = r.publish.resolveHead(commit)
	}
	if fetched, isFetched := r.publish.fetchedTag(revision); isFetched {
		commit = fetched.commit
	}
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

// RemoteHasTag reports whether the tag is among remoteTags or, once HEAD is pushed, among the
// tags the workflow has made by this poll.
func (r memRepo) RemoteHasTag(_ context.Context, tag string) (bool, error) {
	if err := r.failure(methodRemoteHasTag); err != nil {
		return false, err
	}
	if _, hasTag := r.remoteTags[tag]; hasTag {
		return true, nil
	}
	return r.publish.pollTag(tag), nil
}

// RemoteTagType returns the tag's object type from remoteTags or the tags the workflow made.
func (r memRepo) RemoteTagType(_ context.Context, tag string) (string, error) {
	if err := r.failure(methodRemoteTagType); err != nil {
		return "", err
	}
	remote, hasTag := r.remoteTag(tag)
	if !hasTag {
		return "", fmt.Errorf("origin has no tag %s", tag)
	}
	return remote.kind, nil
}

// ReleaseAssets returns the release's assets from releases, and whether drafts holds it.
func (r memRepo) ReleaseAssets(_ context.Context, tag string) (githubReleaseAssets, bool, error) {
	if err := r.failure(methodReleaseAssets); err != nil {
		return githubReleaseAssets{}, false, err
	}
	assets, isFound := r.releases[tag]
	return githubReleaseAssets{names: slices.Clone(assets), isDraft: r.drafts[tag]}, isFound, nil
}

// PushMain records the push of commit in publish and its log; from then on origin/main is
// commit.
func (r memRepo) PushMain(_ context.Context, commit string) error {
	if err := r.failure(methodPushMain); err != nil {
		return err
	}
	if r.publish == nil {
		return errors.New("this memRepo pushes nothing")
	}
	r.publish.note("PushMain", commit)
	r.publish.mu.Lock()
	defer r.publish.mu.Unlock()
	r.publish.isPushed = true
	r.publish.pushedCommit = commit
	return nil
}

// FetchTag records the fetch in publish and its log; until it, Resolve and IsAnnotated know
// nothing of the tag.
func (r memRepo) FetchTag(_ context.Context, tag string) error {
	if err := r.failure(methodFetchTag); err != nil {
		return err
	}
	if r.publish == nil {
		return errors.New("this memRepo fetches nothing")
	}
	r.publish.note("FetchTag", tag)
	remote, hasTag := r.remoteTag(tag)
	if !hasTag {
		return fmt.Errorf("origin has no tag %s", tag)
	}
	r.publish.mu.Lock()
	defer r.publish.mu.Unlock()
	if r.publish.fetched == nil {
		r.publish.fetched = map[string]memTag{}
	}
	r.publish.fetched[tag] = remote
	return nil
}

// IsAnnotated reports whether the fetched tag's object is a tag object.
func (r memRepo) IsAnnotated(_ context.Context, tag string) (bool, error) {
	if err := r.failure(methodIsAnnotated); err != nil {
		return false, err
	}
	fetched, isFetched := r.publish.fetchedTag(tag)
	if !isFetched {
		return false, fmt.Errorf("the checkout has no tag %s", tag)
	}
	return fetched.kind == gitObjectTag, nil
}

// ShowFile returns the file from files; it holds HEAD's files only, so another revision is
// refused.
func (r memRepo) ShowFile(_ context.Context, revision, path string) (string, error) {
	if err := r.failure(methodShowFile); err != nil {
		return "", err
	}
	if r.resolve(revision) != r.head {
		return "", fmt.Errorf("this memRepo holds the files at HEAD only, not at %s", revision)
	}
	content, hasFile := r.files[path]
	if !hasFile {
		return "", fmt.Errorf("HEAD has no %s", path)
	}
	return content, nil
}

// LastCommitTouching walks the history's first parents from revision to the first commit that
// changes path.
func (r memRepo) LastCommitTouching(_ context.Context, revision, path string) (string, error) {
	if err := r.failure(methodLastCommitTouching); err != nil {
		return "", err
	}
	for commit := r.resolve(revision); ; {
		known, isKnown := r.commits[commit]
		if !isKnown {
			return "", fmt.Errorf("unknown commit %s", commit)
		}
		if slices.Contains(known.changed, path) {
			return commit, nil
		}
		if len(known.parents) == 0 {
			return "", fmt.Errorf("no commit at %s changes %s", revision, path)
		}
		commit = known.parents[0]
	}
}

// CloneAt records the clone in publish and its log and writes writeReleaseTree's checkout of
// publishVersion into dir; origin must hold the tag, which a clone fetches first.
func (r memRepo) CloneAt(_ context.Context, tag, dir string) error {
	if err := r.failure(methodCloneAt); err != nil {
		return err
	}
	if r.publish == nil {
		return errors.New("this memRepo clones nothing")
	}
	r.publish.note(methodCloneAt, tag, dir)
	if _, hasTag := r.remoteTag(tag); !hasTag {
		return fmt.Errorf("origin has no tag %s", tag)
	}
	return writeReleaseTree(dir)
}

// CreateRelease records the release in publish's log and adds it to releases with the assets'
// names, each of which must name a file, as gh requires; origin must hold the tag.
func (r memRepo) CreateRelease(_ context.Context, tag, notesPath string, assets []string) error {
	if err := r.failure(methodCreateRelease); err != nil {
		return err
	}
	if r.publish == nil || r.releases == nil {
		return errors.New("this memRepo creates no release")
	}
	r.publish.note(methodCreateRelease, append([]string{tag, notesPath}, assets...)...)
	if _, hasTag := r.remoteTag(tag); !hasTag {
		return fmt.Errorf("origin has no tag %s", tag)
	}
	if _, isFound := r.releases[tag]; isFound {
		return fmt.Errorf("release %s already exists", tag)
	}
	if err := requireFiles(assets); err != nil {
		return err
	}
	r.releases[tag] = assetNames(assets)
	return nil
}

// UploadAssets records the upload in publish's log and adds the assets' names, each of which
// must name a file, to the release.
func (r memRepo) UploadAssets(_ context.Context, tag string, assets []string) error {
	if err := r.failure(methodUploadAssets); err != nil {
		return err
	}
	if r.publish == nil {
		return errors.New("this memRepo uploads nothing")
	}
	r.publish.note(methodUploadAssets, append([]string{tag}, assets...)...)
	uploaded, isFound := r.releases[tag]
	// A test may hand releases a slice sharing its array with others: append to a copy.
	uploaded = slices.Clone(uploaded)
	if !isFound {
		return fmt.Errorf("there is no release %s", tag)
	}
	if err := requireFiles(assets); err != nil {
		return err
	}
	for _, name := range assetNames(assets) {
		if !slices.Contains(uploaded, name) {
			uploaded = append(uploaded, name)
		}
	}
	r.releases[tag] = uploaded
	return nil
}

// UndraftRelease records the un-draft in publish's log and removes the release from drafts; the
// release must exist.
func (r memRepo) UndraftRelease(_ context.Context, tag string) error {
	if err := r.failure(methodUndraftRelease); err != nil {
		return err
	}
	if r.publish == nil {
		return errors.New("this memRepo publishes no draft")
	}
	r.publish.note(methodUndraftRelease, tag)
	if _, isFound := r.releases[tag]; !isFound {
		return fmt.Errorf("there is no release %s", tag)
	}
	delete(r.drafts, tag)
	return nil
}

// ReadAsset records the read in publish's log, when there is one, and returns the asset's
// content from assetContent; the release must carry the asset.
func (r memRepo) ReadAsset(_ context.Context, tag, name string) (string, error) {
	if err := r.failure(methodReadAsset); err != nil {
		return "", err
	}
	r.publish.note(methodReadAsset, tag, name)
	if !slices.Contains(r.releases[tag], name) {
		return "", fmt.Errorf("release %s has no asset %s", tag, name)
	}
	content, hasContent := r.assetContent[name]
	if !hasContent {
		return "", fmt.Errorf("this memRepo holds no content for %s", name)
	}
	return content, nil
}

// DownloadAsset records the download in publish's log, when there is one, and writes the asset's
// content from assetContent to dir/name; the release must carry the asset.
func (r memRepo) DownloadAsset(_ context.Context, tag, name, dir string) error {
	if err := r.failure(methodDownloadAsset); err != nil {
		return err
	}
	r.publish.note(methodDownloadAsset, tag, name, dir)
	if !slices.Contains(r.releases[tag], name) {
		return fmt.Errorf("release %s has no asset %s", tag, name)
	}
	content, hasContent := r.assetContent[name]
	if !hasContent {
		return fmt.Errorf("this memRepo holds no content for %s", name)
	}
	return os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
}

// assetNames returns each asset path's file name.
func assetNames(assets []string) []string {
	names := make([]string, 0, len(assets))
	for _, asset := range assets {
		names = append(names, filepath.Base(asset))
	}
	return names
}

// remoteTag returns origin's tag: from remoteTags, whose commit is not held, or from the tags
// the workflow made.
func (r memRepo) remoteTag(tag string) (memTag, bool) {
	if kind, hasTag := r.remoteTags[tag]; hasTag {
		return memTag{name: tag, kind: kind}, true
	}
	return r.publish.madeTag(tag)
}

// resolve turns the symbolic names into commits: origin/main is the pushed commit once one is
// pushed, and a tag of tagCommits is its commit.
func (r memRepo) resolve(revision string) string {
	switch revision {
	case "HEAD":
		return r.head
	case remoteMain:
		if pushed, isPushed := r.publish.pushed(); isPushed {
			return pushed
		}
		return r.originMain
	}
	if commit, isTag := r.tagCommits[revision]; isTag {
		return commit
	}
	return revision
}

// note adds the call to the log's call record, unanswered, when there is a log; a nil
// memPublish records nothing.
func (p *memPublish) note(method string, args ...string) {
	if p == nil || p.log == nil {
		return
	}
	p.log.mu.Lock()
	defer p.log.mu.Unlock()
	p.log.calls = append(p.log.calls, recordedCall{name: repoCallName, args: append([]string{method}, args...)})
}

// pushed returns the commit PushMain pushed and whether it ran; a nil memPublish pushed nothing.
func (p *memPublish) pushed() (string, bool) {
	if p == nil {
		return "", false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pushedCommit, p.isPushed
}

// resolveHead counts one answer for HEAD and returns head, or headAfterGate after the first
// answer when it is set; a nil memPublish answers head.
func (p *memPublish) resolveHead(head string) string {
	if p == nil {
		return head
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.headResolves++
	if p.headAfterGate != "" && p.headResolves > 1 {
		return p.headAfterGate
	}
	return head
}

// pollTag is one look at origin for tag after the push: it counts the poll and asks the
// workflow, keeping a tag the workflow made. Before the push the workflow has made nothing.
func (p *memPublish) pollTag(tag string) bool {
	if p == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.isPushed {
		return false
	}
	if _, isMade := p.madeTags[tag]; isMade {
		return true
	}
	p.polls++
	if p.tagAfterPush == nil {
		return false
	}
	made, isMade := p.tagAfterPush(p.polls)
	if !isMade || made.name != tag {
		return false
	}
	if p.madeTags == nil {
		p.madeTags = map[string]memTag{}
	}
	p.madeTags[tag] = made
	return true
}

// madeTag returns a tag the workflow made; a nil memPublish made none.
func (p *memPublish) madeTag(tag string) (memTag, bool) {
	if p == nil {
		return memTag{}, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	made, isMade := p.madeTags[tag]
	return made, isMade
}

// fetchedTag returns a tag FetchTag fetched; a nil memPublish fetched none.
func (p *memPublish) fetchedTag(tag string) (memTag, bool) {
	if p == nil {
		return memTag{}, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	fetched, isFetched := p.fetched[tag]
	return fetched, isFetched
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
