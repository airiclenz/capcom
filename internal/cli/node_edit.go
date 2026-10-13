package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/nodestore"
)

const (
	// stdinPath is the file path that makes readTextInput read standard input.
	stdinPath = "-"
	// kindIssueWrite is the --json kind of every command that writes an issue.
	kindIssueWrite = "issue-write"
)

// editIssue changes the issue arg names through the node store of the issue tree, which resolves
// arg through the index, refuses with conflict when ifMatch is set and is not the file's etag,
// applies change to the parsed issue, then validates and writes the result atomically (Kind.Edit).
// It returns the issue written: its id, path and new etag.
func (a *app) editIssue(
	arg string,
	ifMatch string,
	change func(issue.Issue) (issue.Issue, error),
) (nodestore.Written, error) {
	return editNode(nodestore.Issues(a.store, issueResolver{index: a.issueIndex}), arg, ifMatch, change)
}

// editNode runs kind.Edit and types its failure through kindFailure. A failure of change that names
// no file is placed at the node's file, so a refusal of the change itself still names the file.
func editNode[T any](
	kind *nodestore.Kind[T],
	arg string,
	ifMatch string,
	change func(T) (T, error),
) (nodestore.Written, error) {
	var changeErr error
	written, err := kind.Edit(arg, ifMatch, func(node T) (T, error) {
		edited, err := change(node)
		changeErr = err
		return edited, err
	})
	if err == nil {
		return written, nil
	}
	failure := kindFailure(err)
	if err == changeErr && failure.Path == "" {
		failure.Path = written.Path
	}
	return nodestore.Written{}, failure
}

// issueResolver resolves an issue argument through the index: the Resolver of the issue tree's
// node store.
type issueResolver struct {
	index *index.Index
}

// Resolve resolves arg to the issue it names, failing as index.Index.Resolve does.
func (r issueResolver) Resolve(arg string) (nodestore.Target, error) {
	entry, err := r.index.Resolve(arg)
	if err != nil {
		return nodestore.Target{}, err
	}
	return nodestore.Target{ID: entry.ID, Path: entry.Path}, nil
}

// readTextInput reads the file at path, or stdin when path is "-". Errors name the file.
func readTextInput(cmd *cobra.Command, path string) (string, error) {
	if path == stdinPath {
		data, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return "", Errorf(CodeInvalid, "read stdin: %v", err)
		}
		return string(data), nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", &Error{Code: CodeNotFound, Path: path, Message: "no such file"}
	}
	if err != nil {
		return "", &Error{Code: CodeInvalid, Path: path, Message: err.Error()}
	}
	return string(data), nil
}

// writeResult is the result of a command that wrote an issue: its id, path and new etag.
func writeResult(id string, path string, etag string) Result {
	return Result{
		Kind: kindIssueWrite,
		Data: map[string]string{"id": id, "path": path, "etag": etag},
		Text: fmt.Sprintf("id: %s\npath: %s\netag: %s\n", id, path, etag),
		KV:   []KVRecord{{Pairs: []KVPair{{Key: "ID", Value: id}, {Key: "PATH", Value: path}, {Key: "ETAG", Value: etag}}}},
	}
}
