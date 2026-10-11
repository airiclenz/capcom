package cli

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/tree"
)

// failureSeparator joins the messages of several failed lookups into one failure.
const failureSeparator = "; "

// newShowCommand prints each named issue in full. Every argument is resolved on its own; the
// issues found are printed even when another argument fails, and the command then exits with the
// first failure's code. Changes to the shown issues on other local branches are reported apart
// from them (Elsewhere).
func (a *app) newShowCommand() *cobra.Command {
	var isElsewhereSkipped bool
	cmd := &cobra.Command{
		Use:         "show <id|alias|prefix>...",
		Short:       "Show issues in full",
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			result, shownPaths, err := a.showResult(args)
			if err != nil {
				return err
			}
			return a.emit(cmd, a.addElsewhere(result, isElsewhereSkipped, shownPaths))
		}),
	}
	cmd.Flags().BoolVar(&isElsewhereSkipped, flagNoElsewhere, false, "skip the report of changes to these issues on other branches")
	return a.withKV(cmd)
}

// showResult looks up every argument and builds the result: the issues found, a --kv record per
// argument closed by its own STATUS, and the joined failure of the arguments that failed. It also
// returns the paths of the issues found, relative to the worktree root.
func (a *app) showResult(args []string) (Result, []string, error) {
	result := Result{Kind: "issue", KV: []KVRecord{}}
	data := make([]issueData, 0, len(args))
	var shownPaths []string
	var text strings.Builder
	var messages []string
	for _, arg := range args {
		entry, err := a.issueIndex.Resolve(arg)
		if err != nil {
			failure, isArgument := resolveFailure(err)
			if !isArgument {
				return Result{}, nil, failure
			}
			if result.Failure == nil {
				result.Failure = failure
			}
			messages = append(messages, failure.Message)
			result.KV = append(result.KV, KVRecord{
				Pairs:  []KVPair{{"ID", arg}, {kvKeyMessage, failure.Message}},
				Status: string(failure.Code),
			})
			continue
		}

		body, err := a.issueIndex.Body(entry.Path)
		if err != nil {
			return Result{}, nil, err
		}
		file, err := a.readIssueFile(entry.Path)
		if err != nil {
			return Result{}, nil, err
		}
		etag := edit.ETag(file)
		if text.Len() > 0 {
			text.WriteString("\n")
		}
		text.WriteString(issueText(entry, etag, body))
		shown := newIssueData(entry)
		shown.ETag = etag
		shown.Body = &body
		data = append(data, shown)
		result.KV = append(result.KV, issueKV(entry, etag))
		shownPaths = append(shownPaths, entry.Path)
	}
	if result.Failure != nil {
		result.Failure.Message = strings.Join(messages, failureSeparator)
	}
	result.Data = data
	result.Text = text.String()
	return result, shownPaths, nil
}

// resolveFailure types a failure to resolve an argument: no match is not_found, an ambiguous
// prefix is invalid and names the candidates, anything else is the index failing. isArgument is
// false for the last, which a multi-id command cannot go on from.
func resolveFailure(err error) (failure *Error, isArgument bool) {
	switch {
	case errors.Is(err, tree.ErrNotFound):
		return Errorf(CodeNotFound, "%s", err.Error()), true
	case errors.Is(err, tree.ErrAmbiguous):
		return Errorf(CodeInvalid, "%s", err.Error()), true
	default:
		return asError(err), false
	}
}

// resolveIssue resolves one argument to its issue, typing the failure as resolveFailure does.
func (a *app) resolveIssue(arg string) (index.Entry, error) {
	entry, err := a.issueIndex.Resolve(arg)
	if err != nil {
		failure, _ := resolveFailure(err) // either way the command stops here
		return index.Entry{}, failure
	}
	return entry, nil
}
