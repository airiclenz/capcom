package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// repo is the semantic seam over the capcom checkout and its GitHub repository: what the release
// verbs ask of them, in their own terms rather than as git or gh argv. execRepo is the production
// adapter; the tests hold the same answers as plain state in memRepo. Every method fails with an
// error naming the operation when its answer cannot be read; a "no" answer is never an error.
type repo interface {
	// FetchMain fetches origin's main branch and its tags into the checkout.
	FetchMain(ctx context.Context) error
	// IsClean reports whether the working tree has no change and no untracked file.
	IsClean(ctx context.Context) (bool, error)
	// Resolve returns the full commit id a revision — a ref, a tag, a commit id — names.
	Resolve(ctx context.Context, revision string) (string, error)
	// Parents returns commit's parents, first parent first: none for a root commit, two or more
	// for a merge.
	Parents(ctx context.Context, commit string) ([]string, error)
	// ChangedPaths returns the paths commit changes against parent.
	ChangedPaths(ctx context.Context, parent, commit string) ([]string, error)
	// IsAncestor reports whether ancestor is descendant or one of its ancestors.
	IsAncestor(ctx context.Context, ancestor, descendant string) (bool, error)
	// HasGreenRun reports whether ci.yml has at least one successful run for commit.
	HasGreenRun(ctx context.Context, commit string) (bool, error)
	// ReleaseTags returns the checkout's release tags — every tag whose name starts with v — in
	// no particular order.
	ReleaseTags(ctx context.Context) ([]string, error)
	// CommitFile writes content to the checkout's path, keeping the file's mode when it exists,
	// stages that one path by name — never every change — and commits it with subject.
	CommitFile(ctx context.Context, path string, content []byte, subject string) error
	// CountCommits returns how many commits revisionRange holds: `<tag>..HEAD`, or HEAD for the
	// whole history.
	CountCommits(ctx context.Context, revisionRange string) (int, error)
	// LatestRelease returns the latest published GitHub release, and false while none is
	// published.
	LatestRelease(ctx context.Context) (githubRelease, bool, error)
	// RemoteHasTag reports whether origin holds tag.
	RemoteHasTag(ctx context.Context, tag string) (bool, error)
	// RemoteTagType returns the type of the object GitHub's tag ref points at: "tag" for an
	// annotated tag, "commit" for a lightweight one.
	RemoteTagType(ctx context.Context, tag string) (string, error)
	// ReleaseAssets returns the names of the GitHub release's fully uploaded assets — an asset in
	// any other state is a failed or unfinished upload, not one the release carries — and whether
	// the release is still a draft, and false when there is no release for tag.
	ReleaseAssets(ctx context.Context, tag string) (githubReleaseAssets, bool, error)
	// PushMain pushes HEAD to origin's main branch.
	PushMain(ctx context.Context) error
	// FetchTag fetches origin's tag into the checkout's tag of the same name.
	FetchTag(ctx context.Context, tag string) error
	// IsAnnotated reports whether the checkout's tag is an annotated tag rather than a
	// lightweight one.
	IsAnnotated(ctx context.Context, tag string) (bool, error)
	// ShowFile returns the content of the checkout's path as revision holds it.
	ShowFile(ctx context.Context, revision, path string) (string, error)
	// LastCommitTouching returns the last commit, at revision or before it, that changed path.
	LastCommitTouching(ctx context.Context, revision, path string) (string, error)
	// CloneAt fetches origin's tags into the checkout, then clones the checkout fresh into dir —
	// an absolute path, which must not exist yet — and checks tag out there.
	CloneAt(ctx context.Context, tag, dir string) error
	// CreateRelease creates the GitHub release for tag, which origin must already hold, titled
	// tag, with the notes file's text and every asset file, and marks it the latest release.
	CreateRelease(ctx context.Context, tag, notesPath string, assets []string) error
	// UploadAssets uploads each asset file to tag's GitHub release, replacing an asset of the
	// same name.
	UploadAssets(ctx context.Context, tag string, assets []string) error
	// UndraftRelease publishes tag's draft GitHub release and marks it the latest release.
	UndraftRelease(ctx context.Context, tag string) error
	// ReadAsset returns the content of tag's GitHub release asset name, writing nothing to disk.
	ReadAsset(ctx context.Context, tag, name string) (string, error)
	// DownloadAsset downloads tag's GitHub release asset name into the directory dir, as
	// dir/name.
	DownloadAsset(ctx context.Context, tag, name, dir string) error
}

// githubRelease is a published GitHub release: its tag and when it was published.
type githubRelease struct {
	Tag       string
	Published string
}

// githubReleaseAssets is what a GitHub release carries: the names of its fully uploaded assets,
// and whether it is still a draft — one an interrupted `gh release create` left unpublished, whose
// download URLs answer 404 until it is published.
type githubReleaseAssets struct {
	names   []string
	isDraft bool
}

// repoOpener opens the repo for the checkout at dir. newRootCommand hands one to every verb's run
// options, so a test can put any repo behind a whole command.
type repoOpener func(dir string) repo

// execRepos opens execRepo adapters that run their commands through commands.
func execRepos(commands runner) repoOpener {
	return func(dir string) repo {
		return execRepo{commands: commands, dir: dir}
	}
}

// open opens the repo for dir: through o, or, when o is nil — a verb's run options built without
// one, as a test driving a run function directly builds them — the execRepo over commands.
func (o repoOpener) open(commands runner, dir string) repo {
	if o == nil {
		return execRepo{commands: commands, dir: dir}
	}
	return o(dir)
}

// execRepo is the production repo: it owns the git and gh argv for the checkout at dir and for
// its GitHub repository, and turns their output and exit statuses into answers. Every call that
// reaches a remote — a fetch, a push, an ls-remote, any gh call — runs with nonInteractiveEnv, so
// a missing credential fails the call instead of prompting.
type execRepo struct {
	commands runner
	dir      string
}

// git runs a local git command in the checkout.
func (r execRepo) git(ctx context.Context, args ...string) (string, error) {
	return r.commands.run(ctx, r.dir, nil, "git", args...)
}

// remote runs a git or gh command that reaches a remote, never prompting.
func (r execRepo) remote(ctx context.Context, name string, args ...string) (string, error) {
	return r.commands.run(ctx, r.dir, nonInteractiveEnv, name, args...)
}

// FetchMain fetches origin's main and every tag.
func (r execRepo) FetchMain(ctx context.Context) error {
	_, err := r.remote(ctx, "git", "fetch", "--quiet", remoteName, mainBranch, "--tags")
	return err
}

// IsClean reads `git status --porcelain`: a clean tree prints nothing.
func (r execRepo) IsClean(ctx context.Context) (bool, error) {
	status, err := r.git(ctx, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(status) == "", nil
}

// Resolve peels revision to a commit with `git rev-parse --verify`.
func (r execRepo) Resolve(ctx context.Context, revision string) (string, error) {
	output, err := r.git(ctx, "rev-parse", "--verify", revision+"^{commit}")
	if err != nil {
		return "", err
	}
	commit := strings.TrimSpace(output)
	if commit == "" {
		return "", fmt.Errorf("git rev-parse printed no commit for %s", revision)
	}
	return commit, nil
}

// Parents reads `git rev-list --parents -n1`, which prints the commit followed by each of its
// parents.
func (r execRepo) Parents(ctx context.Context, commit string) ([]string, error) {
	output, err := r.git(ctx, "rev-list", "--parents", "-n1", commit)
	if err != nil {
		return nil, err
	}
	fields := strings.Fields(output)
	if len(fields) == 0 {
		return nil, fmt.Errorf("git rev-list printed nothing for %s", commit)
	}
	return fields[1:], nil
}

// ChangedPaths lists the paths with the two-tree `git diff-tree -r --name-only`: the one-commit
// form would print the commit id first, and nothing at all for a merge.
func (r execRepo) ChangedPaths(ctx context.Context, parent, commit string) ([]string, error) {
	output, err := r.git(ctx, "diff-tree", "-r", "--name-only", parent, commit)
	if err != nil {
		return nil, err
	}
	return strings.Fields(output), nil
}

// exitCoder is what a failed command's error carries when the command ran and exited non-zero
// (exec.ExitError has it), so a caller can tell a "no" answer from a command that broke.
type exitCoder interface {
	ExitCode() int
}

// IsAncestor asks `git merge-base --is-ancestor`, which answers with its exit status: 0 yes, 1
// no, anything else a failure.
func (r execRepo) IsAncestor(ctx context.Context, ancestor, descendant string) (bool, error) {
	_, err := r.git(ctx, "merge-base", "--is-ancestor", ancestor, descendant)
	if err == nil {
		return true, nil
	}
	var exit exitCoder
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

// HasGreenRun lists ci.yml's successful runs for commit with `gh run list`.
func (r execRepo) HasGreenRun(ctx context.Context, commit string) (bool, error) {
	output, err := r.remote(ctx, "gh", "run", "list",
		"--workflow", ciWorkflow, "--commit", commit, "--status", "success", "--json", "databaseId")
	if err != nil {
		return false, err
	}
	var runs []struct {
		DatabaseID int64 `json:"databaseId"`
	}
	if err := json.Unmarshal([]byte(output), &runs); err != nil {
		return false, fmt.Errorf("parse gh run list output: %w", err)
	}
	return len(runs) > 0, nil
}

// ReleaseTags lists the tags with `git tag --list v*`, one per line.
func (r execRepo) ReleaseTags(ctx context.Context) ([]string, error) {
	output, err := r.git(ctx, "tag", "--list", "v*")
	if err != nil {
		return nil, err
	}
	return strings.Fields(output), nil
}

// CommitFile writes the file, then runs `git add <path>` and `git commit --quiet -m <subject>`.
func (r execRepo) CommitFile(ctx context.Context, path string, content []byte, subject string) error {
	target := filepath.Join(r.dir, path)
	mode := os.FileMode(documentMode)
	if info, err := os.Stat(target); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.WriteFile(target, content, mode); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if _, err := r.git(ctx, "add", path); err != nil {
		return fmt.Errorf("stage %s: %w", path, err)
	}
	if _, err := r.git(ctx, "commit", "--quiet", "-m", subject); err != nil {
		return fmt.Errorf("commit %q: %w", subject, err)
	}
	return nil
}

// CountCommits counts with `git rev-list --count`.
func (r execRepo) CountCommits(ctx context.Context, revisionRange string) (int, error) {
	output, err := r.git(ctx, "rev-list", "--count", revisionRange)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(output))
}

// LatestRelease asks `gh release view` with no tag, which answers with the latest published
// release, or fails with "release not found" while none is.
func (r execRepo) LatestRelease(ctx context.Context) (githubRelease, bool, error) {
	output, err := r.remote(ctx, "gh", "release", "view", "--repo", capcomRepo, "--json", "tagName,publishedAt")
	if isReleaseNotFound(err) {
		return githubRelease{}, false, nil
	}
	if err != nil {
		return githubRelease{}, false, err
	}
	var release struct {
		TagName     string `json:"tagName"`
		PublishedAt string `json:"publishedAt"`
	}
	if err := json.Unmarshal([]byte(output), &release); err != nil {
		return githubRelease{}, false, fmt.Errorf("parse gh release view output: %w", err)
	}
	if release.TagName == "" {
		return githubRelease{}, false, errors.New("gh release view named no latest release")
	}
	return githubRelease{Tag: release.TagName, Published: release.PublishedAt}, true, nil
}

// RemoteHasTag asks `git ls-remote --tags` for the one tag ref: a missing tag prints nothing.
func (r execRepo) RemoteHasTag(ctx context.Context, tag string) (bool, error) {
	listed, err := r.remote(ctx, "git", "ls-remote", "--tags", remoteName, "refs/tags/"+tag)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(listed) != "", nil
}

// RemoteTagType reads the tag ref's object type from the GitHub API.
func (r execRepo) RemoteTagType(ctx context.Context, tag string) (string, error) {
	kind, err := r.remote(ctx, "gh", "api", "repos/"+capcomRepo+"/git/ref/tags/"+tag, "--jq", ".object.type")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(kind), nil
}

// ReleaseAssets reads the release's assets, their states and whether it is a draft with
// `gh release view <tag>`.
func (r execRepo) ReleaseAssets(ctx context.Context, tag string) (githubReleaseAssets, bool, error) {
	output, err := r.remote(ctx, "gh", "release", "view", tag, "--repo", capcomRepo, "--json", "assets,isDraft")
	if isReleaseNotFound(err) {
		return githubReleaseAssets{}, false, nil
	}
	if err != nil {
		return githubReleaseAssets{}, false, err
	}
	var release struct {
		Assets []struct {
			Name  string `json:"name"`
			State string `json:"state"`
		} `json:"assets"`
		IsDraft bool `json:"isDraft"`
	}
	if err := json.Unmarshal([]byte(output), &release); err != nil {
		return githubReleaseAssets{}, false, fmt.Errorf("parse gh release view output: %w", err)
	}

	uploaded := make([]string, 0, len(release.Assets))
	for _, asset := range release.Assets {
		if asset.State == "uploaded" {
			uploaded = append(uploaded, asset.Name)
		}
	}
	return githubReleaseAssets{names: uploaded, isDraft: release.IsDraft}, true, nil
}

// PushMain pushes with `git push origin HEAD:main`.
func (r execRepo) PushMain(ctx context.Context) error {
	_, err := r.remote(ctx, "git", "push", remoteName, "HEAD:"+mainBranch)
	return err
}

// FetchTag fetches the one tag ref onto itself with `git fetch`.
func (r execRepo) FetchTag(ctx context.Context, tag string) error {
	ref := "refs/tags/" + tag
	_, err := r.remote(ctx, "git", "fetch", "--quiet", remoteName, ref+":"+ref)
	return err
}

// IsAnnotated reads the tag ref's object type with `git cat-file -t`: an annotated tag is a tag
// object, a lightweight one names its commit directly.
func (r execRepo) IsAnnotated(ctx context.Context, tag string) (bool, error) {
	kind, err := r.git(ctx, "cat-file", "-t", "refs/tags/"+tag)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(kind) == gitObjectTag, nil
}

// ShowFile reads the file with `git show <revision>:<path>`.
func (r execRepo) ShowFile(ctx context.Context, revision, path string) (string, error) {
	return r.git(ctx, "show", revision+":"+path)
}

// LastCommitTouching asks `git log -1` limited to path for the commit's full id.
func (r execRepo) LastCommitTouching(ctx context.Context, revision, path string) (string, error) {
	output, err := r.git(ctx, "log", "-1", "--format=%H", revision, "--", path)
	if err != nil {
		return "", err
	}
	commit := strings.TrimSpace(output)
	if commit == "" {
		return "", fmt.Errorf("git log found no commit changing %s at %s", path, revision)
	}
	return commit, nil
}

// CloneAt runs `git fetch --tags`, finds the checkout's top level with `git rev-parse`, clones it
// with `git clone` from dir's parent and runs `git checkout <tag>` in the clone.
func (r execRepo) CloneAt(ctx context.Context, tag, dir string) error {
	if _, err := r.remote(ctx, "git", "fetch", "--quiet", "--tags", remoteName); err != nil {
		return fmt.Errorf("fetch %s's tags: %w", remoteName, err)
	}
	toplevel, err := r.git(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("find the checkout's top level: %w", err)
	}
	if _, err := r.commands.run(ctx, filepath.Dir(dir), nil, "git", "clone", "--quiet", strings.TrimSpace(toplevel), dir); err != nil {
		return fmt.Errorf("clone the checkout to build %s: %w", tag, err)
	}
	if _, err := r.commands.run(ctx, dir, nil, "git", "checkout", "--quiet", tag); err != nil {
		return fmt.Errorf("check out %s in the clone: %w", tag, err)
	}
	return nil
}

// CreateRelease runs `gh release create` with --verify-tag, so gh refuses a tag origin lacks
// rather than making one, and --latest.
func (r execRepo) CreateRelease(ctx context.Context, tag, notesPath string, assets []string) error {
	args := []string{"release", "create", tag, "--repo", capcomRepo,
		"--verify-tag", "--title", tag, "--notes-file", notesPath, "--latest"}
	_, err := r.remote(ctx, "gh", append(args, assets...)...)
	return err
}

// UploadAssets runs `gh release upload --clobber`, which replaces an asset of the same name.
func (r execRepo) UploadAssets(ctx context.Context, tag string, assets []string) error {
	args := []string{"release", "upload", tag, "--repo", capcomRepo, "--clobber"}
	_, err := r.remote(ctx, "gh", append(args, assets...)...)
	return err
}

// UndraftRelease runs `gh release edit --draft=false --latest`, which publishes the draft and
// marks it latest, as CreateRelease's --latest does for a release it creates.
func (r execRepo) UndraftRelease(ctx context.Context, tag string) error {
	_, err := r.remote(ctx, "gh", "release", "edit", tag, "--repo", capcomRepo, "--draft=false", "--latest")
	return err
}

// ReadAsset runs `gh release download --output -`, which prints the asset to stdout, so nothing
// is written into the checkout, whose clean tree the next status or publish gate checks.
func (r execRepo) ReadAsset(ctx context.Context, tag, name string) (string, error) {
	return r.remote(ctx, "gh", "release", "download", tag, "--repo", capcomRepo, "--pattern", name, "--output", "-")
}

// DownloadAsset runs `gh release download --dir`, which writes the asset into dir under its own
// name.
func (r execRepo) DownloadAsset(ctx context.Context, tag, name, dir string) error {
	_, err := r.remote(ctx, "gh", "release", "download", tag, "--repo", capcomRepo, "--pattern", name, "--dir", dir)
	return err
}
