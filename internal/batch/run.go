package batch

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"time"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/nodestore"
	"github.com/airiclenz/capcom/internal/ops"
	"github.com/airiclenz/capcom/internal/stage"
	"github.com/airiclenz/capcom/internal/tree"
)

// closeCommand is the command whose status rules a close operation follows, as a refusal names it.
const closeCommand = "close"

// ErrIfMatchCreated refuses an if-match on an issue an earlier operation of the batch created: its
// file did not exist before the batch, so it has no etag to match.
var ErrIfMatchCreated = errors.New("if-match names an issue this batch creates, which has no etag before the batch")

// OpError is the failure of one operation of a batch: Index is its place in the file, counted from
// 1 (its Position), Line the file line it starts on, Op its Kind and Err the failure, as the write
// it ran returned it (an *ops.ValueError, an *ops.ChangeError, a *nodestore.ConflictError, a tree
// resolution error, ...).
type OpError struct {
	Index int
	Line  int
	Op    Kind
	Err   error
}

// Error renders "operation <n> (<op>): <failure>", as Parse words a problem in an operation.
func (e *OpError) Error() string {
	return fmt.Sprintf("operation %d (%s): %v", e.Index, e.Op, e.Err)
}

// Unwrap returns the operation's failure, so errors.Is and errors.As see through to it.
func (e *OpError) Unwrap() error {
	return e.Err
}

// CategoryError refuses a close to a declared status outside the closed categories, worded as
// capcom close words it: Status is the status, Category its category and Command the command
// whose rule it breaks.
type CategoryError struct {
	Status   string
	Category config.Category
	Command  string
}

// Error names the status, its category and the categories the command takes.
func (e *CategoryError) Error() string {
	return fmt.Sprintf(
		"status %q is in the %s category; %s takes a status in the %v categories",
		e.Status,
		e.Category,
		e.Command,
		config.ClosedCategories(),
	)
}

// Result is the outcome of one operation: the issue it wrote, with the Effects of the write (the
// frontmatter comments it dropped among them), and the warnings its command gives. Problems,
// IsBodyEmpty are set by a create only; MissingPlan by a create, or an update that sets a plan,
// whose plan is in none of the plans directories.
type Result struct {
	nodestore.Written
	// Position is the operation's place in the file, counted from 1, and Kind its operation.
	Position int
	Kind     Kind
	// Problems are the tree problems a create found that do not concern the new issue.
	Problems tree.Errors
	// IsBodyEmpty says whether a create left the new issue's body empty or only white space.
	IsBodyEmpty bool
	// MissingPlan is the message capcom check reports for the plan the operation set, "" when the
	// plan resolves to a file or the operation set none.
	MissingPlan string
}

// Outcome is what a batch did: one Result per operation, in file order; NowReady, the issues that
// existed before the batch, were not Ready then and are Ready after it, in tree order and never
// nil; and RaisedFormat, the store format the batch raised config.yaml to, reported once for the
// whole batch, 0 when it raised none.
type Outcome struct {
	Results      []Result
	NowReady     []string
	RaisedFormat int
}

// Run runs operations in order through o, the staged ops over ov (ops.Staged with ov's Catalog),
// and returns what they did; it never commits: every write stays in ov until its Plan is applied.
// One at serves the whole batch: a create and a close record it in UTC truncated to the second, a
// log dates its entry with at in local time, as capcom log does. A close without a status takes the
// first done status; one naming an undeclared status is an *ops.ValueError and one outside the
// closed categories a *CategoryError, before the issue is read. An if-match is compared with the
// etag the issue's file had on disk before the batch (stage.Overlay.OriginalETag), a mismatch being
// a *nodestore.ConflictError, and one on an issue an earlier operation created is
// ErrIfMatchCreated; the write itself runs with no if-match. Closing an issue already Closed, by
// the batch or before it, fails. The first failure stops the run and returns as an *OpError naming
// the operation.
func Run(o *ops.Ops, ov *stage.Overlay, operations []Op, at time.Time) (Outcome, error) {
	cfg := o.Store().Config
	before, err := o.IssueNodes()
	if err != nil {
		return Outcome{}, err
	}
	r := runner{ops: o, overlay: ov, cfg: cfg, at: at}
	outcome := Outcome{Results: make([]Result, 0, len(operations))}
	for _, op := range operations {
		result, err := r.run(op)
		if err != nil {
			return Outcome{}, &OpError{Index: op.Position, Line: op.Line, Op: op.Kind, Err: err}
		}
		result.Position = op.Position
		result.Kind = op.Kind
		outcome.RaisedFormat = max(outcome.RaisedFormat, result.RaisedFormat)
		outcome.Results = append(outcome.Results, result)
	}
	after, err := o.IssueNodes()
	if err != nil {
		return Outcome{}, err
	}
	outcome.NowReady = nowReady(before, after, cfg)
	return outcome, nil
}

// runner runs the operations of one batch.
type runner struct {
	ops     *ops.Ops
	overlay *stage.Overlay
	cfg     config.Config
	at      time.Time
	// closer closes the batch's issues; the first close makes it.
	closer *ops.Closer
}

// run runs one operation and checks its if-match.
func (r *runner) run(op Op) (Result, error) {
	result, err := r.write(op)
	if op.IfMatch != "" && result.Path != "" {
		if conflict := r.checkIfMatch(result.Path, op.IfMatch); conflict != nil {
			return Result{}, conflict
		}
	}
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

// write runs the write op names with no if-match. Once the issue resolved, the Result names its
// file even when the write fails.
func (r *runner) write(op Op) (Result, error) {
	switch op.Kind {
	case KindCreate:
		return r.create(op)
	case KindUpdate:
		return r.update(op)
	case KindDepAdd:
		written, err := r.ops.AddDep(op.ID, "", op.Dep)
		return Result{Written: written}, err
	case KindClose:
		return r.close(op)
	case KindLog:
		written, err := r.ops.AppendLog(op.ID, "", op.Text, r.at.Local())
		return Result{Written: written}, err
	default:
		return Result{}, fmt.Errorf("unknown operation %q", op.Kind)
	}
}

// create writes the new issue op describes, created at the batch's time.
func (r *runner) create(op Op) (Result, error) {
	spec := op.Create
	spec.Created = r.timestamp()
	created, err := r.ops.CreateIssue(spec)
	if err != nil {
		return Result{}, err
	}
	return Result{
		Written:     created.Written,
		Problems:    created.Problems,
		IsBodyEmpty: created.IsBodyEmpty,
		MissingPlan: created.MissingPlan,
	}, nil
}

// update applies op's change; one that changes nothing rewrites the issue as it is, as capcom
// update does.
func (r *runner) update(op Op) (Result, error) {
	var written nodestore.Written
	var err error
	if op.Update.IsEmpty() {
		written, err = r.ops.EditIssue(op.ID, "", func(target issue.Issue) (issue.Issue, error) {
			return target, nil
		})
	} else {
		written, err = r.ops.UpdateIssue(op.ID, "", op.Update)
	}
	if err != nil {
		return Result{Written: written}, err
	}
	result := Result{Written: written}
	if op.Update.Plan != nil {
		result.MissingPlan = r.ops.MissingPlan(*op.Update.Plan)
	}
	return result, nil
}

// close closes op's issue at the batch's time, to op's status or the first done status.
func (r *runner) close(op Op) (Result, error) {
	status, err := r.closeStatus(op.Status)
	if err != nil {
		return Result{}, err
	}
	if r.closer == nil {
		closer, err := r.ops.NewCloser()
		if err != nil {
			return Result{}, err
		}
		r.closer = closer
	}
	closed, err := r.closer.Close(op.ID, "", status, op.Reason, r.timestamp())
	return Result{Written: closed.Written}, err
}

// closeStatus returns the status a close moves its issue to: given, which must be declared and in
// a closed category, or the first done status when given is "".
func (r *runner) closeStatus(given string) (string, error) {
	if given == "" {
		status, _ := r.cfg.DefaultStatus(config.CategoryDone)
		return status, nil
	}
	if err := ops.CheckStatus(r.cfg, given); err != nil {
		return "", err
	}
	category, _ := r.cfg.CategoryOf(given)
	if !slices.Contains(config.ClosedCategories(), category) {
		return "", &CategoryError{Status: given, Category: category, Command: closeCommand}
	}
	return given, nil
}

// checkIfMatch compares ifMatch with the etag the file at path, relative to the working tree root
// with forward slashes, had before the batch: nil when they match, a *nodestore.ConflictError when
// they do not, ErrIfMatchCreated when an earlier operation created the file.
func (r *runner) checkIfMatch(path string, ifMatch string) error {
	absolute := filepath.Join(r.ops.Store().WorktreeRoot, filepath.FromSlash(path))
	etag, err := r.overlay.OriginalETag(absolute)
	if errors.Is(err, fs.ErrNotExist) {
		if _, statErr := r.overlay.Stat(absolute); statErr == nil {
			return fmt.Errorf("%s: %w", path, ErrIfMatchCreated)
		}
	}
	if err != nil {
		return err
	}
	if etag == ifMatch {
		return nil
	}
	return &nodestore.ConflictError{
		Path:    path,
		IfMatch: ifMatch,
		ETag:    etag,
		Message: fmt.Sprintf("etag %s does not match the file's etag %s; show the issue again", ifMatch, etag),
	}
}

// timestamp returns the batch's time as a create and a close record it: UTC, truncated to the
// second.
func (r *runner) timestamp() time.Time {
	return r.at.UTC().Truncate(time.Second)
}

// nowReady returns the ids of the issues in after that are Ready and not hidden, that before holds
// and did not judge Ready, in after's order, never nil.
func nowReady(before []issue.Node, after []issue.Node, cfg config.Config) []string {
	wasReady := map[string]bool{}
	isExisting := map[string]bool{}
	for _, node := range before {
		isExisting[node.ID] = true
	}
	for _, verdict := range issue.Ready(before, cfg) {
		wasReady[verdict.ID] = verdict.IsReady
	}
	ready := []string{}
	for _, verdict := range issue.Ready(after, cfg) {
		if verdict.IsReady && !verdict.IsHidden && isExisting[verdict.ID] && !wasReady[verdict.ID] {
			ready = append(ready, verdict.ID)
		}
	}
	return ready
}
