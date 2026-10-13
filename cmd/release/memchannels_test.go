package main

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
)

// The channelRepos methods a memChannelRepos reads through and can be told to fail.
const (
	methodExists      = "Exists"
	methodReadFile    = "ReadFile"
	methodListDirs    = "ListDirs"
	methodOpenPulls   = "OpenPulls"
	methodPublishFile = "PublishFile"
)

// channelsCallName is the name a memChannelRepos' writes are recorded under in a runner's call
// record.
const channelsCallName = "channels"

// memChannelRepos is the in-memory channelRepos: the tap, the bucket and winget-pkgs held as
// plain state — which repositories exist, the files and directories each holds, its open pull
// requests — and the error each method fails with on each repository, so a test states what the
// channel repositories hold instead of what gh prints. Its zero value holds no repository: every
// read answers "no", and PublishFile is refused.
type memChannelRepos struct {
	// repos are the repositories that exist, by name.
	repos map[string]memChannelRepo
	// failures are the errors methods fail with, by method and repository, before anything is
	// read: an error wrapping errUnparsableAnswer stands for an answer the adapter could not
	// parse.
	failures map[memChannelCall]error
	// record is what the methods read and PublishFile wrote; nil records nothing and refuses
	// PublishFile. It is a pointer so every copy of the memChannelRepos records into one.
	record *memChannelRecord
}

// memChannelRepo is one channel repository: its files and the directories under each path, both
// by path on its default branch, and its open pull requests. An empty repository holds no file.
type memChannelRepo struct {
	files map[string]string
	dirs  map[string][]string
	pulls []pullRequest
}

// memChannelCall is one method called on one repository.
type memChannelCall struct {
	method string
	repo   string
}

// memChannelPublish is one file PublishFile wrote: where, what, and the subject of the commit
// that pushed it.
type memChannelPublish struct {
	repo    string
	path    string
	content string
	subject string
}

// memChannelRecord is the part of a memChannelRepos its calls change.
type memChannelRecord struct {
	// log is the runner whose ordered call record PublishFile joins, as calls named
	// channelsCallName, so a test reads the channels' writes and the commands the runner ran in
	// one order; nil records nothing there.
	log *fakeRunner

	mu sync.Mutex
	// reads are the reads made, in order.
	reads []memChannelCall
	// published are the files PublishFile pushed, in order; later reads see them.
	published []memChannelPublish
}

// withFailure returns a copy of the repositories whose method fails with err on repo.
func (c memChannelRepos) withFailure(method, repo string, err error) memChannelRepos {
	failures := maps.Clone(c.failures)
	if failures == nil {
		failures = map[memChannelCall]error{}
	}
	failures[memChannelCall{method: method, repo: repo}] = err
	c.failures = failures
	return c
}

// recording returns a copy of the repositories that records into a fresh record.
func (c memChannelRepos) recording() memChannelRepos {
	c.record = &memChannelRecord{}
	return c
}

// reads returns the reads made so far; none when nothing is recorded.
func (c memChannelRepos) reads() []memChannelCall {
	if c.record == nil {
		return nil
	}
	c.record.mu.Lock()
	defer c.record.mu.Unlock()
	return slices.Clone(c.record.reads)
}

// read records the read and returns the error the method was told to fail with on repo.
func (c memChannelRepos) read(method, repo string) error {
	call := memChannelCall{method: method, repo: repo}
	if c.record != nil {
		c.record.mu.Lock()
		c.record.reads = append(c.record.reads, call)
		c.record.mu.Unlock()
	}
	return c.failures[call]
}

// Exists reports whether repos holds the repository.
func (c memChannelRepos) Exists(_ context.Context, repo string) (bool, error) {
	if err := c.read(methodExists, repo); err != nil {
		return false, err
	}
	_, isFound := c.repos[repo]
	return isFound, nil
}

// ReadFile returns the file PublishFile last pushed to path, else the one the repository holds.
func (c memChannelRepos) ReadFile(_ context.Context, repo, path string) (string, bool, error) {
	if err := c.read(methodReadFile, repo); err != nil {
		return "", false, err
	}
	content, isFound := c.file(repo, path)
	return content, isFound, nil
}

// ListDirs returns the directories the repository holds under path.
func (c memChannelRepos) ListDirs(_ context.Context, repo, path string) ([]string, bool, error) {
	if err := c.read(methodListDirs, repo); err != nil {
		return nil, false, err
	}
	dirs, isFound := c.repos[repo].dirs[path]
	return slices.Clone(dirs), isFound, nil
}

// OpenPulls returns every open pull request the repository holds, whatever the query: the
// probes judge each pull request's title themselves, as they must of the viewer's pull requests
// the production adapter adds unsearched.
func (c memChannelRepos) OpenPulls(_ context.Context, repo, _ string) ([]pullRequest, error) {
	if err := c.read(methodOpenPulls, repo); err != nil {
		return nil, err
	}
	return slices.Clone(c.repos[repo].pulls), nil
}

// PublishFile records the write in record and its log and pushes it — reporting true — only when
// content differs from what the repository holds at path; the repository must exist.
func (c memChannelRepos) PublishFile(_ context.Context, repo, path, content, subject string) (bool, error) {
	if err := c.failures[memChannelCall{method: methodPublishFile, repo: repo}]; err != nil {
		return false, err
	}
	if c.record == nil {
		return false, errors.New("this memChannelRepos publishes nothing")
	}
	if _, isFound := c.repos[repo]; !isFound {
		return false, fmt.Errorf("clone %s: no such repository", repo)
	}
	if held, isFound := c.file(repo, path); isFound && held == content {
		return false, nil
	}

	c.record.mu.Lock()
	c.record.published = append(c.record.published,
		memChannelPublish{repo: repo, path: path, content: content, subject: subject})
	c.record.mu.Unlock()
	if log := c.record.log; log != nil {
		log.mu.Lock()
		log.calls = append(log.calls, recordedCall{name: channelsCallName, args: []string{methodPublishFile, repo, path, subject}})
		log.mu.Unlock()
	}
	return true, nil
}

// file returns what path holds in the repository: the content PublishFile last pushed there, else
// the repository's own file.
func (c memChannelRepos) file(repo, path string) (string, bool) {
	if c.record != nil {
		c.record.mu.Lock()
		defer c.record.mu.Unlock()
		for _, write := range slices.Backward(c.record.published) {
			if write.repo == repo && write.path == path {
				return write.content, true
			}
		}
	}
	content, isFound := c.repos[repo].files[path]
	return content, isFound
}
