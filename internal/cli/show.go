package cli

import (
	"errors"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/tree"
)

// failureSeparator joins the messages of several failed lookups into one failure.
const failureSeparator = "; "

// newShowCommand prints each named issue in full. Every argument is resolved on its own; the
// issues found are printed even when another argument fails, and the command then exits with the
// first failure's code. Changes to the shown issues on other local and remote-tracking branches
// are reported apart from them (Elsewhere).
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
	data := make([]showData, 0, len(args))
	var graph *showGraph
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
		related, err := a.showLinks(entry)
		if err != nil {
			return Result{}, nil, err
		}
		if graph == nil {
			if graph, err = a.loadShowGraph(); err != nil {
				return Result{}, nil, err
			}
		}
		facts := graph.facts(entry.ID, related)
		etag := edit.ETag(file)
		if text.Len() > 0 {
			text.WriteString("\n")
		}
		text.WriteString(issueText(entry, etag, facts, body))
		shown := newShowData(entry, facts)
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

// showGraph is what show needs from the whole issue tree: the ready rules' verdict on each issue in
// the todo category, by id, and the Dependency edges.
type showGraph struct {
	verdicts map[string]issue.Readiness
	deps     issue.DepGraph
}

// loadShowGraph judges every issue in the index, Closed ones included, once for all of show's
// arguments.
func (a *app) loadShowGraph() (*showGraph, error) {
	nodes, err := a.issueNodes()
	if err != nil {
		return nil, err
	}
	verdicts := issue.Ready(nodes, a.store.Config)
	graph := &showGraph{
		verdicts: make(map[string]issue.Readiness, len(verdicts)),
		deps:     issue.NewDepGraph(nodes),
	}
	for _, verdict := range verdicts {
		graph.verdicts[verdict.ID] = verdict
	}
	return graph, nil
}

// facts gathers what show tells about the issue with id beyond its file; related is its links as
// showLinks lists them.
func (g *showGraph) facts(id string, related []string) showFacts {
	facts := showFacts{related: related, dependents: nonNil(g.deps.Dependents(id))}
	if verdict, isJudged := g.verdicts[id]; isJudged {
		facts.readiness = &verdict
	}
	return facts
}

// showLinks lists an issue's Related links for show: its own related entries in the file's order,
// as the file writes them, then the ids of the issues whose links name it, in id order. Each
// appears once; an issue with no links yields an empty list.
func (a *app) showLinks(entry index.Entry) ([]string, error) {
	linkedBy, err := a.issueIndex.RelatedBy(entry.ID)
	if err != nil {
		return nil, err
	}
	links := make([]string, 0, len(entry.Related)+len(linkedBy))
	for _, link := range slices.Concat(entry.Related, linkedBy) {
		if !slices.Contains(links, link) {
			links = append(links, link)
		}
	}
	return links, nil
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
