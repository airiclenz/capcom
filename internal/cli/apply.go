package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/batch"
	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/nodestore"
	"github.com/airiclenz/capcom/internal/ops"
	"github.com/airiclenz/capcom/internal/reflist"
	"github.com/airiclenz/capcom/internal/stage"
)

const (
	// flagDryRun runs the batch without committing it.
	flagDryRun = "dry-run"
	// kindApply is the --json kind of capcom apply.
	kindApply = "apply"
	// applyStdinName names the apply file read from stdin in a failure, as batch.Parse names it.
	applyStdinName = "stdin"
	// dryRunLine opens the text of a dry run.
	dryRunLine = "dry run: nothing written"
)

// applyOutcomes is the outcome each operation reports once it has run.
var applyOutcomes = map[batch.Kind]string{
	batch.KindCreate: "created",
	batch.KindUpdate: "updated",
	batch.KindDepAdd: "dep_added",
	batch.KindClose:  outcomeClosed,
	batch.KindLog:    "logged",
}

// The help text of capcom apply.
const (
	applyLong = "Apply an apply file: a YAML list of Issue writes, run in order and written all or none. " +
		"Give its path, or - to read it from stdin.\n\n" +
		"Each entry is a map whose op key names the write (create, update, dep add, close or log) and " +
		"whose other keys are that command's flag names without the leading --, plus id, dep and text. " +
		"Values are inline: body-file and body-append-file are refused.\n\n" +
		"update, dep add, close and log take if-match, compared with the Issue's file as it was before " +
		"the batch; it is refused on an Issue an earlier operation created.\n\n" +
		"The writes are staged in memory and land together once every one has passed. The first that " +
		"fails stops the batch, naming its operation and line, and nothing is written. Closing an Issue " +
		"already Closed fails the batch.\n\n" +
		"--dry-run runs the same batch and reports the same outcome, but writes nothing.\n\n" +
		"It prints one line per operation: its position, op, Issue id and outcome, then a \"now ready:\" " +
		"line naming the Issues that existed before the batch and are Ready after it."
	applyExample = "  capcom apply batch.yaml             # every operation of batch.yaml, or none\n" +
		"  capcom apply batch.yaml --dry-run   # the outcome, with nothing written\n" +
		"  capcom apply - < batch.yaml         # the apply file from stdin\n" +
		"  capcom apply batch.yaml --json      # the outcome in a JSON envelope"
)

// applyData is the --json data of capcom apply.
type applyData struct {
	IsDryRun bool             `json:"dry_run"`
	Ops      []applyOperation `json:"ops"`
	NowReady []string         `json:"now_ready"`
}

// applyOperation is one operation of the batch as it ran: its position from 1, its line, its op,
// the Issue it wrote, its outcome and the etag the Issue's file has after it.
type applyOperation struct {
	Index   int    `json:"index"`
	Line    int    `json:"line"`
	Op      string `json:"op"`
	ID      string `json:"id"`
	Outcome string `json:"outcome"`
	ETag    string `json:"etag"`
}

// newApplyCommand applies an apply file of Issue writes, all or none.
func (a *app) newApplyCommand() *cobra.Command {
	isDryRun := false
	cmd := &cobra.Command{
		Use:         "apply <file|->",
		Short:       "Apply a file of issue writes, all or none",
		Long:        applyLong,
		Example:     applyExample,
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			result, err := a.apply(cmd, args[0], isDryRun)
			if err != nil {
				return err
			}
			return a.emit(cmd, result)
		}),
	}
	cmd.Flags().BoolVar(&isDryRun, flagDryRun, false, "run the batch and report its outcome, writing nothing")
	return a.withKV(cmd)
}

// apply reads the apply file at path ("-" for stdin), parses it, runs its operations against the
// store staged in memory (batch.Run) and, unless isDryRun, commits every staged write as one plan.
// A file that does not parse is invalid at its line, and an operation that fails is typed by
// operationFailure; either way nothing is written. The writes are noted for the Tree lock warning
// only once committed, so a dry run never warns of the lock.
func (a *app) apply(cmd *cobra.Command, path string, isDryRun bool) (output.Result, error) {
	data, err := readApplyFile(cmd, path)
	if err != nil {
		return output.Result{}, err
	}
	operations, err := batch.Parse(path, data)
	var parseError *batch.Error
	if errors.As(err, &parseError) {
		return output.Result{}, &output.Error{
			Code:    output.CodeInvalid,
			Path:    parseError.Name,
			Line:    parseError.Line,
			Message: parseError.Message,
		}
	}
	if err != nil {
		return output.Result{}, err
	}

	overlay := stage.New(a.store)
	catalog := overlay.Catalog(a.issueIndex)
	staged := ops.Staged(a.store, overlay, overlay.RaiseFormat, catalog)
	outcome, err := batch.Run(staged, overlay, operations, time.Now())
	if err != nil {
		return output.Result{}, a.operationFailure(applyFileName(path), operations, catalog, err)
	}
	if !isDryRun {
		if err := a.commitBatch(overlay, outcome); err != nil {
			return output.Result{}, err
		}
	}
	result := applyResult(operations, outcome, isDryRun)
	result.Warnings = a.batchWarnings(outcome)
	return result, nil
}

// readApplyFile returns the apply file at path, or stdin when path is "-". A missing file is
// not_found at it, hinting the apply file on stdin.
func readApplyFile(cmd *cobra.Command, path string) ([]byte, error) {
	if path == stdinPath {
		data, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return nil, output.Errorf(output.CodeInvalid, "read stdin: %v", err)
		}
		return data, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, &output.Error{
			Code:    output.CodeNotFound,
			Path:    path,
			Message: "no such file",
			Hint:    "capcom apply " + stdinPath + " < <file>",
		}
	}
	if err != nil {
		return nil, &output.Error{Code: output.CodeInvalid, Path: path, Message: err.Error()}
	}
	return data, nil
}

// applyFileName names the apply file at path in a failure: path itself, or stdin for "-".
func applyFileName(path string) string {
	if path == stdinPath {
		return applyStdinName
	}
	return path
}

// operationFailure types err, the failure of batch.Run, for the apply file name names. An
// *batch.OpError is placed at the file and the operation's line, with the code of the failure its
// command reports and that failure, its own file and line kept, after "operation <n> (<op>): ".
// Any other failure is returned as it is.
func (a *app) operationFailure(name string, operations []batch.Op, catalog ops.Catalog, err error) error {
	var failed *batch.OpError
	if !errors.As(err, &failed) {
		return err
	}
	cause := a.operationCause(operations[failed.Index-1], catalog, failed.Err)
	return &output.Error{
		Code:        cause.Code,
		Path:        name,
		Line:        failed.Line,
		Message:     fmt.Sprintf("operation %d (%s): %s", failed.Index, failed.Op, cause.Error()),
		Suggestion:  cause.Suggestion,
		Suggestions: cause.Suggestions,
		Valid:       cause.Valid,
		Hint:        cause.Hint,
	}
}

// operationCause types err, the failure of op, as op's own command types it: a create as capcom
// create does, a value the ops checks refuse as the flag that sets it, a dep add refused by
// reflist at the dependent's file as capcom dep add words it, and every other failure as
// writeFailure types an Issue write's. catalog resolves the dependent's file as the batch staged
// it.
func (a *app) operationCause(op batch.Op, catalog ops.Catalog, err error) *output.Error {
	if op.Kind == batch.KindCreate {
		return output.AsError(a.createFailure(err, op.ID))
	}
	var value *ops.ValueError
	if errors.As(err, &value) {
		return output.AsError(valueFailure(err, invalidErrorf))
	}
	var refusal *reflist.Refusal
	if op.Kind == batch.KindDepAdd && errors.As(err, &refusal) {
		dependentPath := ""
		if entry, resolveErr := catalog.Resolve(op.ID); resolveErr == nil {
			dependentPath = entry.Path
		}
		words := refusalWords{self: depSelfFormat, cycle: depCycleFormat}
		return a.refusalFailure(err, issueIDs, dependentPath, words)
	}
	return a.writeFailure(issueIDs, op.ID, err)
}

// commitBatch makes every write overlay staged on disk as one plan, all of them or none, and notes
// each written Issue for the Tree lock warning (app.noteWrite), so a lock held for any other
// purpose warns once for the whole batch. A plan refused because a file changed since the batch
// read it, or a file appeared where the batch creates one, is typed by nodestore's CommitFailure
// and kindFailure; a cleanup failure of the plan is a write warning.
func (a *app) commitBatch(overlay *stage.Overlay, outcome batch.Outcome) error {
	warnings, err := overlay.Plan().Apply()
	if err != nil {
		var created []nodestore.Written
		for _, result := range outcome.Results {
			if result.Kind == batch.KindCreate {
				created = append(created, result.Written)
			}
		}
		return a.kindFailure(nodestore.Issues(a.store, nil).CommitFailure(err, created), issueIDs)
	}
	for _, warning := range warnings {
		a.writeWarnings = append(a.writeWarnings, a.worktreeRelativeText(warning.Error()))
	}
	for _, result := range outcome.Results {
		a.noteWrite(result.ID)
	}
	return nil
}

// batchWarnings returns the warnings the batch's writes give, each once, as their commands give
// them: the tree problems a create found that the index refresh did not already warn about, a
// create that left its body empty, and a plan set that is in none of the plans directories. It
// adds the frontmatter comments the writes dropped and the store format the batch raised, once, to
// the write warnings.
func (a *app) batchWarnings(outcome batch.Outcome) []string {
	var warnings []string
	addOnce := func(found ...string) {
		for _, warning := range found {
			if !slices.Contains(warnings, warning) {
				warnings = append(warnings, warning)
			}
		}
	}
	for _, result := range outcome.Results {
		addOnce(a.unwarnedTreeProblems(result.Problems)...)
		if result.IsBodyEmpty {
			addOnce(fmt.Sprintf(emptyBodyWarning, result.ID, result.ID))
		}
		addOnce(missingPlanWarnings(result.Path, result.MissingPlan)...)
		a.warnDroppedComments(result.Effects)
	}
	a.warnRaisedFormat(nodestore.Effects{RaisedFormat: outcome.RaisedFormat})
	return warnings
}

// applyResult is the result of a batch: in text a line "<n>  <op>  <id>  <outcome>" per operation,
// then "now ready: " and the Issues the batch made Ready, when there are any, a dry run opening
// with dryRunLine; in --json the applyData; in --kv a record per operation.
func applyResult(operations []batch.Op, outcome batch.Outcome, isDryRun bool) output.Result {
	data := applyData{
		IsDryRun: isDryRun,
		Ops:      make([]applyOperation, 0, len(outcome.Results)),
		NowReady: outcome.NowReady,
	}
	records := make([]output.KVRecord, 0, len(outcome.Results))
	var text strings.Builder
	if isDryRun {
		fmt.Fprintln(&text, dryRunLine)
	}
	for index, result := range outcome.Results {
		operation := applyOperation{
			Index:   result.Position,
			Line:    operations[index].Line,
			Op:      string(result.Kind),
			ID:      result.ID,
			Outcome: applyOutcomes[result.Kind],
			ETag:    result.ETag,
		}
		data.Ops = append(data.Ops, operation)
		fmt.Fprintf(&text, "%d  %s  %s  %s\n", operation.Index, operation.Op, operation.ID, operation.Outcome)
		records = append(records, output.KVRecord{
			Pairs: []output.KVPair{
				{Key: "INDEX", Value: strconv.Itoa(operation.Index)},
				{Key: "LINE", Value: strconv.Itoa(operation.Line)},
				{Key: "OP", Value: operation.Op},
				{Key: "ID", Value: operation.ID},
				{Key: "ETAG", Value: operation.ETag},
			},
			Status: operation.Outcome,
		})
	}
	if len(outcome.NowReady) > 0 {
		fmt.Fprintf(&text, "now ready: %s\n", strings.Join(outcome.NowReady, ", "))
	}
	return output.Result{Kind: kindApply, Data: data, Text: text.String(), KV: records}
}
