package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/nodestore"
	"github.com/airiclenz/capcom/internal/ops"
)

const (
	// stdinPath is the file path that makes readTextInput read standard input.
	stdinPath = "-"
	// kindIssueWrite is the --json kind of every command that writes an issue.
	kindIssueWrite = "issue-write"
)

// ops returns the shared writes (internal/ops) on the issues of the store, resolving references
// through the issue index.
func (a *app) ops() *ops.Ops {
	return ops.New(a.store, a.issueIndex)
}

// editIssue changes the issue arg names through ops.EditIssue, which resolves arg through the
// index, refuses with conflict when ifMatch is set and is not the file's etag, applies change to the
// parsed issue, then validates and writes the result atomically (Kind.Edit). It returns the issue
// written: its id, path and new etag; a failure is typed as written types it.
func (a *app) editIssue(
	arg string,
	ifMatch string,
	change func(issue.Issue) (issue.Issue, error),
) (nodestore.Written, error) {
	return a.written(a.ops().EditIssue(arg, ifMatch, change))
}

// editNode runs kind.Edit through ops.EditNode and types its outcome as written does.
func editNode[T any](
	a *app,
	kind *nodestore.Kind[T],
	arg string,
	ifMatch string,
	change func(T) (T, error),
) (nodestore.Written, error) {
	return a.written(ops.EditNode(kind, arg, ifMatch, change))
}

// written types the outcome of an ops write for the command. An edit that raised the store
// format, because the file it wrote carries a key line config.yaml's format lacks, adds
// formatRaisedWarning to the warnings the command's result renders, so every edit verb warns of
// the raise once. A failure is typed by writeFailure.
func (a *app) written(written nodestore.Written, err error) (nodestore.Written, error) {
	if err != nil {
		return nodestore.Written{}, writeFailure(err)
	}
	a.warnRaisedFormat(written.Effects)
	return written, nil
}

// writeFailure types the failure of an ops write. A refusal of the change itself (*ops.ChangeError)
// is worded by statusChangeFailure when it is a status rule's refusal, and by kindFailure
// otherwise, and placed at the node's file when it names no file, so a refusal of the change still
// names the file. Any other failure is typed by kindFailure.
func writeFailure(err error) *output.Error {
	var refusal *ops.ChangeError
	if !errors.As(err, &refusal) {
		return kindFailure(err)
	}
	var statusRefusal *issue.StatusError
	failure := kindFailure(refusal.Err)
	if errors.As(refusal.Err, &statusRefusal) {
		failure = output.AsError(statusChangeFailure(refusal.Err))
	}
	if failure.Path == "" {
		failure.Path = refusal.Path
	}
	return failure
}

// readTextInput reads the file at path, or stdin when path is "-". Errors name the file.
func readTextInput(cmd *cobra.Command, path string) (string, error) {
	if path == stdinPath {
		data, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return "", output.Errorf(output.CodeInvalid, "read stdin: %v", err)
		}
		return string(data), nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", &output.Error{Code: output.CodeNotFound, Path: path, Message: "no such file"}
	}
	if err != nil {
		return "", &output.Error{Code: output.CodeInvalid, Path: path, Message: err.Error()}
	}
	return string(data), nil
}

// writeResult is the result of a command that wrote an issue: its id, path and new etag.
func writeResult(id string, path string, etag string) output.Result {
	return output.Result{
		Kind: kindIssueWrite,
		Data: map[string]string{"id": id, "path": path, "etag": etag},
		Text: fmt.Sprintf("id: %s\npath: %s\netag: %s\n", id, path, etag),
		KV:   []output.KVRecord{{Pairs: []output.KVPair{{Key: "ID", Value: id}, {Key: "PATH", Value: path}, {Key: "ETAG", Value: etag}}}},
	}
}
