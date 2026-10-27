package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/nodestore"
	"github.com/airiclenz/capcom/internal/ops"
)

const (
	// stdinPath is the file path that makes readTextInput read standard input.
	stdinPath = "-"
	// kindIssueWrite is the --json kind of every command that writes an issue.
	kindIssueWrite = "issue-write"
	// droppedCommentsWarning is the warning of a write that rewrote a file whose frontmatter held a
	// YAML comment (app.warnDroppedComments): the file, relative to the working tree root.
	droppedCommentsWarning = "%s: frontmatter comments are not kept; capcom writes frontmatter in canonical form"
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
	edited, err := a.ops().EditIssue(arg, ifMatch, change)
	return a.written(issueIDs, edited, err)
}

// editNode runs kind.Edit through ops.EditNode and types its outcome as written does for idKind,
// the kind of node kind holds.
func editNode[T any](
	a *app,
	kind *nodestore.Kind[T],
	idKind idKind,
	arg string,
	ifMatch string,
	change func(T) (T, error),
) (nodestore.Written, error) {
	edited, err := ops.EditNode(kind, arg, ifMatch, change)
	return a.written(idKind, edited, err)
}

// written types the outcome of an ops write for the command. An edit that raised the store
// format, because the file it wrote carries a key line config.yaml's format lacks, adds
// formatRaisedWarning to the warnings the command's result renders, so every edit verb warns of
// the raise once, and an edit of a file whose frontmatter held a comment adds
// droppedCommentsWarning (app.warnDroppedComments). A failure is typed by writeFailure for kind, the kind of node written, with the
// id the Written still names when the node resolved.
func (a *app) written(kind idKind, written nodestore.Written, err error) (nodestore.Written, error) {
	if err != nil {
		return nodestore.Written{}, a.writeFailure(kind, written.ID, err)
	}
	a.warnRaisedFormat(written.Effects)
	a.warnDroppedComments(written.Effects)
	return written, nil
}

// warnDroppedComments adds droppedCommentsWarning to the command's write warnings for each file
// whose frontmatter comments the write dropped, and nothing when it dropped none.
func (a *app) warnDroppedComments(effects nodestore.Effects) {
	for _, path := range effects.DroppedComments.Paths() {
		a.writeWarnings = append(a.writeWarnings, fmt.Sprintf(droppedCommentsWarning, path))
	}
}

// writeFailure types the failure of an ops write on the node of kind id names. A refusal of the
// change itself (*ops.ChangeError) is worded by statusChangeFailure when it is a status rule's
// refusal, and by kindFailure otherwise, and placed at the node's file when it names no file, so a
// refusal of the change still names the file. Any other failure is typed by kindFailure.
func (a *app) writeFailure(kind idKind, id string, err error) *output.Error {
	var refusal *ops.ChangeError
	if !errors.As(err, &refusal) {
		return a.kindFailure(err, kind)
	}
	var statusRefusal *issue.StatusError
	failure := a.kindFailure(refusal.Err, kind)
	if errors.As(refusal.Err, &statusRefusal) {
		failure = output.AsError(statusChangeFailure(id, refusal.Err))
	}
	if failure.Path == "" {
		failure.Path = refusal.Path
	}
	return failure
}

// bodyEdits holds the repeatable body edits of one update: the --body-replace-old texts and the
// --body-replace-new texts that pair with them by position, and the --body-append texts, each in
// the order given.
type bodyEdits struct {
	olds    []string
	news    []string
	appends []string
}

// checkPairs returns a usage error naming both counts when the old and new texts of the
// replacement pairs differ in number; nil when every old text has its new one.
func (e bodyEdits) checkPairs() error {
	if len(e.olds) == len(e.news) {
		return nil
	}
	return output.UsageErrorf(
		"%d --%s and %d --%s given; give them in pairs",
		len(e.olds),
		flagBodyReplaceOld,
		len(e.news),
		flagBodyReplaceNew,
	)
}

// apply returns body with every replacement pair applied in order, each against the body the
// pairs before it left, then every append in order. It fails, returning no body, with the usage
// error of checkPairs when the pairs are uneven, and as invalid naming the pair by its position
// from 1 and its old text when that text is empty or does not occur exactly once, so a failed
// edit writes nothing.
func (e bodyEdits) apply(body string) (string, error) {
	if err := e.checkPairs(); err != nil {
		return "", err
	}
	for index, old := range e.olds {
		replaced, err := edit.ReplaceBody(body, old, e.news[index])
		if err != nil {
			return "", output.Errorf(
				output.CodeInvalid,
				"--%s #%d %q: %s",
				flagBodyReplaceOld,
				index+1,
				old,
				err.Error(),
			)
		}
		body = replaced
	}
	for _, text := range e.appends {
		body = edit.AppendBody(body, text)
	}
	return body, nil
}

// readTextInput reads the file at path, which the flag named flag gave, or stdin when path is "-".
// Errors name the file; a missing file hints retrying the command with the text on stdin.
func readTextInput(cmd *cobra.Command, flag string, path string) (string, error) {
	if path == stdinPath {
		data, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return "", output.Errorf(output.CodeInvalid, "read stdin: %v", err)
		}
		return string(data), nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", &output.Error{
			Code:    output.CodeNotFound,
			Path:    path,
			Message: "no such file",
			Hint:    "capcom <command> --" + flag + " " + stdinPath,
		}
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
