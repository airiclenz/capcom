// Package ops is the write glue both front ends share (ADR 0012): it joins the in-memory rules of
// `issue` to the atomic, ETag-checked writes of `nodestore`, one function per write. Every write
// resolves a reference through the index, takes an if-match ETag ("" skips the check) and returns a
// typed result, never rendered text; the caller words and renders failures.
//
// The edit contract every write follows: once the reference has resolved, the returned
// nodestore.Written names the node's id and file even when the write fails, its ETag empty. A
// refusal of the change itself (a status rule, a missing Log author) is a *ChangeError at the
// node's file, distinct from the store's own failures (*nodestore.ConflictError,
// *nodestore.FileError, a tree resolution error), which return unwrapped.
package ops

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/gitx"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/nodestore"
	"github.com/airiclenz/capcom/internal/store"
)

// The refusals of a write's own input, returned before anything is read.
var (
	// ErrBlankReason refuses a close or reopen whose reason is empty or only white space.
	ErrBlankReason = errors.New("the reason is empty")
	// ErrBlankLog refuses a Log entry whose text is empty or only white space.
	ErrBlankLog = errors.New("the log text is empty")
	// ErrNoDoingStatus refuses a claim when config.yaml declares no status in the doing category.
	ErrNoDoingStatus = errors.New("config.yaml declares no status in the doing category")
)

// ChangeError is a refusal of the change an edit applied to the parsed node: Path is the node's
// file, relative to the working tree root with forward slashes, and Err the change's own error.
type ChangeError struct {
	Path string
	Err  error
}

// Error names the file and the change's refusal.
func (e *ChangeError) Error() string {
	return fmt.Sprintf("%s: %v", e.Path, e.Err)
}

// Unwrap returns the change's error, so errors.Is and errors.As see through to it.
func (e *ChangeError) Unwrap() error {
	return e.Err
}

// Ops runs the writes on the issues of one open store, resolving references through its index.
// The index is read, never refreshed: the caller refreshes it when it needs the writes reflected.
type Ops struct {
	store *store.Store
	index *index.Index
}

// New returns the writes on the issues of s, resolving references through x.
func New(s *store.Store, x *index.Index) *Ops {
	return &Ops{store: s, index: x}
}

// Edit changes the node ref names through kind (nodestore.Kind.Edit): it checks ifMatch, applies
// change to the parsed node, validates and writes it atomically. It returns the node written; on a
// failure after ref resolved, the Written still names the node's id and file. A failure of change
// is a *ChangeError at that file; every other failure is Kind.Edit's, unwrapped.
func Edit[T any](
	kind *nodestore.Kind[T],
	ref string,
	ifMatch string,
	change func(T) (T, error),
) (nodestore.Written, error) {
	var changeErr error
	written, err := kind.Edit(ref, ifMatch, func(node T) (T, error) {
		edited, err := change(node)
		changeErr = err
		return edited, err
	})
	if err != nil && err == changeErr {
		return written, &ChangeError{Path: written.Path, Err: err}
	}
	return written, err
}

// EditIssue changes the issue ref names as Edit does, through the issue tree's node store.
func (o *Ops) EditIssue(
	ref string,
	ifMatch string,
	change func(issue.Issue) (issue.Issue, error),
) (nodestore.Written, error) {
	return Edit(nodestore.Issues(o.store, issueResolver{index: o.index}), ref, ifMatch, change)
}

// SetStatus moves the issue ref names to status, which must not cross into or out of a closed
// category (issue.SetStatus); a refusal is a *ChangeError wrapping the *issue.StatusError.
func (o *Ops) SetStatus(ref string, ifMatch string, status string) (nodestore.Written, error) {
	return o.EditIssue(ref, ifMatch, func(target issue.Issue) (issue.Issue, error) {
		return issue.SetStatus(target, o.store.Config, status)
	})
}

// Claim moves the issue ref names to the first status of the doing category, as SetStatus does. It
// fails with ErrNoDoingStatus, writing nothing, when config.yaml declares none.
func (o *Ops) Claim(ref string, ifMatch string) (nodestore.Written, error) {
	status, isDeclared := o.store.Config.DefaultStatus(config.CategoryDoing)
	if !isDeclared {
		return nodestore.Written{}, ErrNoDoingStatus
	}
	return o.SetStatus(ref, ifMatch, status)
}

// CloseResult is the outcome of one close: the issue written and the ids of the todo issues the
// close made Ready, in node order, never nil.
type CloseResult struct {
	nodestore.Written
	NewlyReady []string
}

// Close closes the issue ref names, as a Closer of its own does.
func (o *Ops) Close(
	ref string,
	ifMatch string,
	status string,
	reason string,
	at time.Time,
) (CloseResult, error) {
	closer, err := o.NewCloser()
	if err != nil {
		return CloseResult{}, err
	}
	return closer.Close(ref, ifMatch, status, reason, at)
}

// Closer closes the issues of one batch against a single read of the issue tree, so the issues
// each close makes Ready count the closes before it in the batch. It is not safe for concurrent
// use.
type Closer struct {
	ops        *Ops
	nodes      []issue.Node
	closing    map[string]config.Category
	isReported map[string]bool
}

// NewCloser reads every usable issue from the index, Closed ones included, and returns a Closer
// over them. The index is not refreshed after the writes, so the issues are read once, before them.
func (o *Ops) NewCloser() (*Closer, error) {
	nodes, err := o.IssueNodes()
	if err != nil {
		return nil, err
	}
	return &Closer{
		ops:        o,
		nodes:      nodes,
		closing:    map[string]config.Category{},
		isReported: map[string]bool{},
	}, nil
}

// Close moves the issue ref names to status, a status in a closed category, recording at as its
// closed time and reason as why (issue.Close); at is stored as given. A blank reason fails with
// ErrBlankReason, writing nothing. An issue already Closed, judged on its file, is a *ChangeError
// wrapping issue.ErrAlreadyClosed. NewlyReady names the issues this close made Ready given the
// batch's earlier closes, leaving out any an earlier close of the batch already named.
func (c *Closer) Close(
	ref string,
	ifMatch string,
	status string,
	reason string,
	at time.Time,
) (CloseResult, error) {
	if strings.TrimSpace(reason) == "" {
		return CloseResult{}, ErrBlankReason
	}
	cfg := c.ops.store.Config
	written, err := c.ops.EditIssue(ref, ifMatch, func(target issue.Issue) (issue.Issue, error) {
		return issue.Close(target, cfg, status, reason, at)
	})
	if err != nil {
		return CloseResult{Written: written}, err
	}

	category, _ := cfg.CategoryOf(status)
	c.closing[written.ID] = category
	result := CloseResult{Written: written, NewlyReady: []string{}}
	for _, id := range issue.NewlyReady(c.nodes, cfg, c.closing) {
		if !c.isReported[id] {
			c.isReported[id] = true
			result.NewlyReady = append(result.NewlyReady, id)
		}
	}
	return result, nil
}

// Reopen moves the Closed issue ref names to status, a status outside the closed categories,
// clears its closed time and reason, and appends "reopened: <reason>" to its Log, dated at and
// attributed to the author git would record (issue.Reopen). A blank reason fails with
// ErrBlankReason, writing nothing; a refusal of issue.Reopen, or a failure to name the author, is
// a *ChangeError.
func (o *Ops) Reopen(
	ref string,
	ifMatch string,
	status string,
	reason string,
	at time.Time,
) (nodestore.Written, error) {
	if strings.TrimSpace(reason) == "" {
		return nodestore.Written{}, ErrBlankReason
	}
	author := func() (string, error) { return gitx.AuthorName(o.store.WorktreeRoot) }
	return o.EditIssue(ref, ifMatch, func(target issue.Issue) (issue.Issue, error) {
		return issue.Reopen(target, o.store.Config, status, reason, author, at)
	})
}

// AppendLog appends text to the Log of the issue ref names, dated at and attributed to the author
// git would record (gitx.AuthorName). A blank text fails with ErrBlankLog, writing nothing; a
// failure to name the author is a *ChangeError.
func (o *Ops) AppendLog(ref string, ifMatch string, text string, at time.Time) (nodestore.Written, error) {
	if strings.TrimSpace(text) == "" {
		return nodestore.Written{}, ErrBlankLog
	}
	return o.EditIssue(ref, ifMatch, func(target issue.Issue) (issue.Issue, error) {
		author, err := gitx.AuthorName(o.store.WorktreeRoot)
		if err != nil {
			return issue.Issue{}, err
		}
		return target.AppendLog(at, author, text), nil
	})
}

// IssueNodes returns the Node of every usable issue in the index, Closed ones included.
func (o *Ops) IssueNodes() ([]issue.Node, error) {
	entries, err := o.index.List(index.Filter{IsClosedIncluded: true})
	if err != nil {
		return nil, err
	}
	return index.Nodes(entries), nil
}

// issueResolver resolves an issue reference through the index: the Resolver of the issue tree's
// node store.
type issueResolver struct {
	index *index.Index
}

// Resolve resolves ref to the issue it names, failing as index.Index.Resolve does.
func (r issueResolver) Resolve(ref string) (nodestore.Target, error) {
	entry, err := r.index.Resolve(ref)
	if err != nil {
		return nodestore.Target{}, err
	}
	return nodestore.Target{ID: entry.ID, Path: entry.Path}, nil
}
