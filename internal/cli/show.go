package cli

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/nodestore"
	"github.com/airiclenz/capcom/internal/tree"
)

// failureSeparator joins the messages of several failed lookups into one failure.
const failureSeparator = "; "

// idSuggestionFormat ends one failed lookup's message with its own suggestions when several
// lookups fail and their messages are joined.
const idSuggestionFormat = "%s, did you mean %s?"

// flagSection makes capcom show print only the body sections under the headings it names.
const flagSection = "section"

// The help text of capcom show.
const (
	showLong = "Print one or more Issues in full: their fields, readiness, dependencies and dependents, child " +
		"progress, Related links, body and Log.\n\n" +
		"Name an Issue by its id, an Alias or a unique id prefix.\n\n" +
		"The etag printed is the value --if-match takes on update, mv and rm.\n\n" +
		"--brief prints the fields only, without the body and its Log. --section prints the fields and the " +
		"body sections under the headings named, in body order, heading lines included; a heading matches " +
		"case-insensitively, its # marks ignored, so --section log selects ## Log.\n\n" +
		"A section runs to the next heading of its level or a higher one. An Issue whose body lacks a " +
		"named heading fails, naming the headings it has; the others still print.\n\n" +
		"Changes to these Issues on other branches are reported as Elsewhere unless --no-elsewhere is " +
		"given."
	showExample = "  capcom show login-timeout-race\n" +
		"  capcom show login-timeout-race refresh-retry   # several at once\n" +
		"  capcom show login-ti                           # a unique id prefix\n" +
		"  capcom show login-timeout-race --brief         # the fields only, no body\n" +
		"  capcom show login-timeout-race --section acceptance --section log\n" +
		"  capcom show login-timeout-race --json          # the versioned JSON envelope"
)

// newShowCommand prints each named issue in full. Every argument is resolved on its own; the
// issues found are printed even when another argument fails, and the command then exits with the
// first failure's code. Changes to the shown issues on other local and remote-tracking branches
// are reported apart from them (Elsewhere). --brief leaves each body out and --section keeps only
// the body sections it names.
func (a *app) newShowCommand() *cobra.Command {
	var isElsewhereSkipped bool
	var view bodyView
	cmd := &cobra.Command{
		Use:               "show <id|alias|prefix>...",
		Short:             "Show issues in full",
		Long:              showLong,
		Example:           showExample,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completeIssueIDs(everyArgument),
		Annotations:       map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			return a.emitShown(cmd, args, isElsewhereSkipped, view, nil)
		}),
	}
	flags := cmd.Flags()
	flags.BoolVar(&isElsewhereSkipped, flagNoElsewhere, false, "skip the report of changes to these issues on other branches")
	flags.BoolVar(&view.isBrief, flagBrief, false, "print the fields only, without the body and its Log")
	flags.StringArrayVar(&view.sections, flagSection, nil,
		"print the fields and only the body section under this heading; repeat for more")
	cmd.MarkFlagsMutuallyExclusive(flagBrief, flagSection)
	return a.withKV(cmd)
}

// bodyView is how much of each issue's or Note's body show and know show print: all of it, the zero
// value; none, with isBrief; or only the sections under the headings in sections, as
// issue.Sections selects them.
type bodyView struct {
	isBrief  bool
	sections []string
}

// body is the part of a body the view prints, and whether it prints any: false with
// isBrief. It fails, as a *issue.SectionsError, when the body lacks a heading the view names.
func (v bodyView) body(full string) (string, bool, error) {
	switch {
	case v.isBrief:
		return "", false, nil
	case len(v.sections) == 0:
		return full, true, nil
	}
	selected, err := issue.Sections(full, v.sections)
	return selected, err == nil, err
}

// emitShown prints the issues args name as show does, each body as view has it, with the Elsewhere
// report of their files unless isElsewhereSkipped. show and next both end here; next passes the
// zero view and prints each issue in full, and the graph its Ready list was judged from, where
// show passes nil (see showResult).
func (a *app) emitShown(
	cmd *cobra.Command, args []string, isElsewhereSkipped bool, view bodyView, graph *showGraph,
) error {
	result, shownPaths, err := a.showResult(cmd, args, view, graph)
	if err != nil {
		return err
	}
	return a.emit(cmd, a.addElsewhere(result, isElsewhereSkipped, shownPaths))
}

// showResult looks up every argument and builds the result: the issues found, a --kv record per
// argument closed by its own STATUS, and the joined failure of the arguments that failed: when
// several fail, each message names its own suggestions and the joined failure carries none. It also
// returns the paths of the issues found, relative to the worktree root. Each issue's fields, body
// and etag come from one read of its file, the bytes a later --if-match is checked against;
// readiness, dependents, child progress and reverse Related links are the index's. Each body is
// printed and given as view has it; an issue whose body lacks a heading view names fails as its
// argument does, invalid. Each failed argument's failure names cmd as the command that failed.
// graph is the whole tree judged already, as next has it from its Ready list; nil judges it here,
// once, when the first argument resolves.
func (a *app) showResult(
	cmd *cobra.Command, args []string, view bodyView, graph *showGraph,
) (output.Result, []string, error) {
	result := output.Result{Kind: "issue", KV: []output.KVRecord{}}
	data := make([]showData, 0, len(args))
	var shownPaths []string
	var text strings.Builder
	failures := a.newArgumentFailures(cmd, issueIDs)
	reader := nodestore.IssueReader(a.store.Config)
	for _, arg := range args {
		entry, err := a.issueIndex.Resolve(arg)
		if err != nil {
			failure, fatal := failures.add(err)
			if fatal != nil {
				return output.Result{}, nil, fatal
			}
			result.KV = append(result.KV, failureKV(arg, failure))
			continue
		}

		reading, err := parseNodeFile(a, reader, entry.Path)
		if err != nil {
			return output.Result{}, nil, err
		}
		entry, fullBody := a.issueFromFile(entry, reading.Node)
		body, hasBody, err := view.body(fullBody)
		if err != nil {
			failure := failures.addFailure(output.Errorf(output.CodeInvalid, "%s: %s", entry.Path, err))
			result.KV = append(result.KV, failureKV(entry.ID, failure))
			continue
		}
		related, err := a.showLinks(entry)
		if err != nil {
			return output.Result{}, nil, err
		}
		if graph == nil {
			if graph, err = a.loadShowGraph(); err != nil {
				return output.Result{}, nil, err
			}
		}
		children, err := a.childProgress(entry.ID)
		if err != nil {
			return output.Result{}, nil, err
		}
		facts := graph.facts(entry.ID, related, children)
		etag := reading.ETag
		if text.Len() > 0 {
			text.WriteString("\n")
		}
		text.WriteString(issueText(entry, etag, facts, body))
		shown := newShowData(entry, facts)
		shown.ETag = etag
		if hasBody {
			shown.Body = &body
		}
		data = append(data, shown)
		result.KV = append(result.KV, issueKV(entry, etag))
		shownPaths = append(shownPaths, entry.Path)
	}
	result.Failure = failures.failure()
	result.Data = data
	result.Text = text.String()
	return result, shownPaths, nil
}

// failureKV is the --kv record of the argument arg that failed: its id, the failure's message and
// hint, closed by the failure's code as its STATUS.
func failureKV(arg string, failure *output.Error) output.KVRecord {
	pairs := []output.KVPair{{Key: "ID", Value: arg}, {Key: output.KVKeyMessage, Value: failure.Message}}
	return output.KVRecord{
		Pairs:  append(pairs, output.ErrorHintPairs(failure)...),
		Status: string(failure.Code),
	}
}

// argumentFailures collects the failures of the arguments a multi-id command could not resolve,
// so that the command goes on to the rest: it exits with the first failure's code, and when several
// fail each message names its own suggestions and the joined failure carries none.
type argumentFailures struct {
	app      *app
	command  string
	kind     idKind
	first    *output.Error
	messages []string
}

// newArgumentFailures returns an empty collection of the failures of cmd's arguments, each naming
// a node of kind.
func (a *app) newArgumentFailures(cmd *cobra.Command, kind idKind) *argumentFailures {
	return &argumentFailures{app: a, command: cmd.CommandPath(), kind: kind}
}

// add types err, the failure to resolve one argument, as resolveFailure does, names the command
// as the one that failed and collects it. It returns the typed failure, or as its error the
// failure the command cannot go on from: the index failing.
func (f *argumentFailures) add(err error) (*output.Error, error) {
	failure, isArgument := f.app.resolveFailure(err, f.kind)
	if !isArgument {
		return nil, failure
	}
	return f.addFailure(failure), nil
}

// addFailure names the command as the one that failed in failure, the typed failure of one
// argument, collects it and returns it.
func (f *argumentFailures) addFailure(failure *output.Error) *output.Error {
	failure = failure.WithCommand(f.command)
	if f.first == nil {
		f.first = failure
	}
	f.messages = append(f.messages, messageWithSuggestions(failure))
	return failure
}

// failure is the failure the command exits with: nil when every argument resolved, else the first
// failure, its message every failure's message joined when several failed.
func (f *argumentFailures) failure() *output.Error {
	if len(f.messages) > 1 {
		f.first.Message = strings.Join(f.messages, failureSeparator)
		f.first.Suggestions = nil
	}
	return f.first
}

// messageWithSuggestions is failure's message ending in its own suggestions, as in
// `id "a" not found, did you mean x?`, so that a joined failure of several lookups names each
// id's suggestions after it; the message alone when it has none.
func messageWithSuggestions(failure *output.Error) string {
	if len(failure.Suggestions) == 0 {
		return failure.Message
	}
	return fmt.Sprintf(idSuggestionFormat, failure.Message, output.JoinAlternatives(failure.Suggestions))
}

// issueFromFile returns entry with every frontmatter field taken from parsed, the issue its file
// parses to, and the body. The entry keeps its id, path and parent, which the file's place in the
// tree gives.
func (a *app) issueFromFile(entry index.Entry, parsed issue.Issue) (index.Entry, string) {
	category, _ := a.store.Config.CategoryOf(parsed.Status) // an undeclared status has none
	return index.Entry{
		ID:         entry.ID,
		Path:       entry.Path,
		ParentID:   entry.ParentID,
		Status:     parsed.Status,
		Category:   category,
		Type:       parsed.Type,
		Priority:   parsed.Priority,
		Plan:       parsed.Plan,
		IsStanding: parsed.IsStanding,
		Created:    parsed.Created,
		Closed:     parsed.Closed,
		Reason:     parsed.Reason,
		Title:      parsed.Title,
		Labels:     parsed.Labels,
		Deps:       parsed.Deps,
		Related:    parsed.Related,
		Aliases:    parsed.Aliases,
	}, parsed.Body
}

// showGraph is what show needs from the whole issue tree: the ready rules' verdict on each issue in
// the todo category, by id, and the Dependency edges.
type showGraph struct {
	verdicts map[string]issue.Readiness
	deps     issue.DepGraph
}

// afterShowGraphLoad runs each time loadShowGraph is called. It is nil in production; tests set
// it to count the loads.
var afterShowGraphLoad func()

// loadShowGraph judges every issue in the index, Closed ones included, once for all of show's
// arguments.
func (a *app) loadShowGraph() (*showGraph, error) {
	if afterShowGraphLoad != nil {
		afterShowGraphLoad()
	}
	nodes, err := a.issueNodes()
	if err != nil {
		return nil, err
	}
	return newShowGraph(issue.Ready(nodes, a.store.Config), issue.NewDepGraph(nodes)), nil
}

// newShowGraph is the showGraph of verdicts, the ready rules' verdicts on the issue tree, and
// deps, its Dependency graph.
func newShowGraph(verdicts []issue.Readiness, deps issue.DepGraph) *showGraph {
	graph := &showGraph{verdicts: make(map[string]issue.Readiness, len(verdicts)), deps: deps}
	for _, verdict := range verdicts {
		graph.verdicts[verdict.ID] = verdict
	}
	return graph
}

// facts gathers what show tells about the issue with id beyond its file; related is its links as
// showLinks lists them, children its direct children's count as childProgress gives it.
func (g *showGraph) facts(id string, related []string, children *childrenData) showFacts {
	facts := showFacts{related: related, dependents: nonNil(g.deps.Dependents(id)), children: children}
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

// resolveFailure types a failure to resolve an argument naming a node of kind: no match is
// not_found, typed by notFoundFailure, an ambiguous prefix is invalid and names the candidates,
// anything else is the index failing. isArgument is false for the last, which a multi-id command
// cannot go on from, and for the index failing to list the names of kind.
func (a *app) resolveFailure(err error, kind idKind) (failure *output.Error, isArgument bool) {
	switch {
	case errors.Is(err, tree.ErrNotFound):
		return a.notFoundFailure(err, kind)
	case errors.Is(err, tree.ErrAmbiguous):
		return output.Errorf(output.CodeInvalid, "%s", err.Error()), true
	default:
		return output.AsError(err), false
	}
}

// resolveIssue resolves one argument to its issue, typing the failure as resolveFailure does for
// an issue.
func (a *app) resolveIssue(arg string) (index.Entry, error) {
	entry, err := a.issueIndex.Resolve(arg)
	if err != nil {
		failure, _ := a.resolveFailure(err, issueIDs) // either way the command stops here
		return index.Entry{}, failure
	}
	return entry, nil
}
