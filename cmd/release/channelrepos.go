package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// channelRepos is the semantic seam over the package channels' GitHub repositories — the tap,
// the bucket and winget-pkgs: what the channel probes ask of them, in their own terms rather than
// as gh argv. execChannelRepos is the production adapter. A "no" answer — a repository, file or
// directory that does not exist, or an empty repository — is never an error; every method fails
// with an error when its answer cannot be read, and one wrapping errUnparsableAnswer when the
// answer came back in a shape it cannot parse.
type channelRepos interface {
	// Exists reports whether the repository exists.
	Exists(ctx context.Context, repo string) (bool, error)
	// ReadFile returns the content of the file at path in the repository's default branch, and
	// whether there is one: false for a missing file, repository or commit.
	ReadFile(ctx context.Context, repo, path string) (string, bool, error)
	// ListDirs returns the names of the directories directly under path in the repository's
	// default branch, and whether path is there to list: false for a missing path, repository or
	// commit.
	ListDirs(ctx context.Context, repo, path string) ([]string, bool, error)
	// OpenPulls returns the repository's open pull requests that a GitHub search for query finds.
	OpenPulls(ctx context.Context, repo, query string) ([]pullRequest, error)
}

// pullRequest is one pull request a channel repository holds.
type pullRequest struct {
	number int
	title  string
	url    string
}

// errUnparsableAnswer marks a channel repository answer the adapter could not parse. The error
// carrying it reads as the parser's own message alone, so a caller wording the failure itself
// keeps the parser's detail.
var errUnparsableAnswer = errors.New("unparsable channel repository answer")

// unparsableAnswerError is a parse failure wrapping errUnparsableAnswer and the parser's error.
type unparsableAnswerError struct {
	cause error
}

func (e unparsableAnswerError) Error() string { return e.cause.Error() }

func (e unparsableAnswerError) Unwrap() []error { return []error{errUnparsableAnswer, e.cause} }

// execChannelRepos is the production channelRepos: it owns the gh argv for the channel
// repositories, run in dir, and reads GitHub's 404 — a missing repository or path, or an empty
// repository — as a "no".
type execChannelRepos struct {
	commands runner
	dir      string
}

// gh runs one gh command in dir.
func (r execChannelRepos) gh(ctx context.Context, args ...string) (string, error) {
	return r.commands.run(ctx, r.dir, nil, "gh", args...)
}

// Exists asks the repositories API for the repository's full name.
func (r execChannelRepos) Exists(ctx context.Context, repo string) (bool, error) {
	_, err := r.gh(ctx, "api", "repos/"+repo, "--jq", ".full_name")
	if isNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ReadFile reads the file through the contents API.
func (r execChannelRepos) ReadFile(ctx context.Context, repo, path string) (string, bool, error) {
	content, err := r.readContents(ctx, repo, path)
	if isNotFound(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return content, true, nil
}

// readContents reads one file through the contents API, failing on a 404 like on any other
// failed call.
func (r execChannelRepos) readContents(ctx context.Context, repo, path string) (string, error) {
	encoded, err := r.gh(ctx, "api", "repos/"+repo+"/contents/"+path, "--jq", ".content")
	if err != nil {
		return "", err
	}
	// The contents API wraps its base64 at 60 columns.
	decoded, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(encoded), ""))
	if err != nil {
		return "", fmt.Errorf("decode %s: %w", path, unparsableAnswerError{cause: err})
	}
	return string(decoded), nil
}

// ListDirs lists path through the contents API and keeps its directory entries.
func (r execChannelRepos) ListDirs(ctx context.Context, repo, path string) ([]string, bool, error) {
	listing, err := r.gh(ctx, "api", "repos/"+repo+"/contents/"+path)
	if isNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var entries []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(listing), &entries); err != nil {
		return nil, false, unparsableAnswerError{cause: err}
	}
	dirs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Type == "dir" {
			dirs = append(dirs, entry.Name)
		}
	}
	return dirs, true, nil
}

// OpenPulls lists the open pull requests with `gh pr list --search`.
func (r execChannelRepos) OpenPulls(ctx context.Context, repo, query string) ([]pullRequest, error) {
	output, err := r.gh(ctx, "pr", "list", "--repo", repo, "--state", "open",
		"--search", query, "--json", "number,title,url")
	if err != nil {
		return nil, err
	}
	var listed []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		URL    string `json:"url"`
	}
	if err := json.Unmarshal([]byte(output), &listed); err != nil {
		return nil, unparsableAnswerError{cause: err}
	}
	pulls := make([]pullRequest, 0, len(listed))
	for _, pull := range listed {
		pulls = append(pulls, pullRequest{number: pull.Number, title: pull.Title, url: pull.URL})
	}
	return pulls, nil
}
