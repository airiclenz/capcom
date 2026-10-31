package cli

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/ops"
	"github.com/airiclenz/capcom/internal/reflist"
)

// Flags of capcom create.
const (
	flagDep        = "dep"
	flagBody       = "body"
	flagWhy        = "why"
	flagAcceptance = "acceptance"
)

// emptyBodyWarning is the warning of a create that leaves the new issue without a body; it takes
// the new issue's id twice.
const emptyBodyWarning = `%s has no body: add what, why and acceptance with capcom update %s --body-append <text>; ` +
	`create takes --body, --why and --acceptance`

// createOptions holds the flags of one capcom create.
type createOptions struct {
	id         string
	title      string
	parent     string
	issueType  string
	priority   int
	labels     []string
	plan       string
	status     string
	body       string
	bodyFile   string
	why        string
	acceptance []string
	deps       []string
	isStanding bool
}

// The help text of capcom create.
const (
	createLong = "Create an Issue under the id you choose: a kebab-case slug of 1 to 4 words that names the " +
		"work.\n\n" +
		"--parent files it below another Issue, and --dep makes it wait for the Issues named; a " +
		"--dep that names no Issue fails the create before anything is written.\n\n" +
		"The body comes from --body, or --body-file (- reads stdin); --why adds a \"## Why\" section and " +
		"each --acceptance a bullet under \"## Acceptance\". An Issue created without a body draws a " +
		"warning.\n\n" +
		"--standing marks it a standing Parent that only groups its children: it is never Ready, and " +
		"it is not claimed or closed while marked, so it cannot be created in the doing category.\n\n" +
		"The first standing Issue in a store below format 5 raises the store format to 5, with a " +
		"warning that older capcom releases can no longer read the store.\n\n" +
		"It prints the new Issue's id, path and etag."
	createExample = "  capcom create --id login-timeout-race --title \"Session refresh races the login timeout\" --type bug\n" +
		"  capcom create --id refresh-retry --title \"Retry a failed token refresh\" --dep login-timeout-race   # waits for it\n" +
		"  capcom create --id login-copy --title \"Reword the login errors\" --parent login-flow --priority 1\n" +
		"  capcom create --id cache-warmup --title \"Warm the cache on deploy\" --body \"Deploys start cold.\" --why \"The first request times out.\" --acceptance \"Warm after deploy\"\n" +
		"  capcom create --id cache-evict --title \"Evict stale cache entries\" --body-file - < body.md   # body from stdin\n" +
		"  capcom create --id feature-requests --title \"Feature requests\" --standing   # only groups its children: never Ready"
)

// newCreateCommand writes a new issue file. The id is always the caller's choice, never generated.
func (a *app) newCreateCommand() *cobra.Command {
	options := &createOptions{}
	cmd := &cobra.Command{
		Use:         "create --id <slug> --title <title>",
		Short:       "Create an issue with a chosen id",
		Long:        createLong,
		Example:     createExample,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			result, err := a.create(cmd, options)
			if err != nil {
				return err
			}
			return a.emit(cmd, result)
		}),
	}
	flags := cmd.Flags()
	flags.StringVar(&options.id, flagID, "", "the new issue's id: a kebab slug of 1 to 4 words (required)")
	flags.StringVar(&options.title, flagTitle, "", "the issue's title (required)")
	flags.StringVar(&options.parent, flagParent, "", "create the issue below this one")
	a.completeFlag(cmd, flagParent, completeIDValue(issueIDs))
	flags.StringVar(&options.issueType, flagType, "", "the issue's type (default task, or the first configured type)")
	a.completeFlag(cmd, flagType, completeTypes)
	flags.IntVar(&options.priority, flagPriority, issue.DefaultPriority, "priority, 0 (most urgent) to 4")
	flags.StringSliceVar(&options.labels, flagLabel, nil, "labels; repeat or separate with commas")
	flags.StringVar(&options.plan, flagPlan, "", "basename of the plan the issue belongs to")
	flags.StringVar(&options.status, flagStatus, "", "initial status, not in a closed category (default: the first todo status)")
	a.completeFlag(cmd, flagStatus, completeStatuses(isOpenCategory))
	flags.StringVar(&options.body, flagBody, "", "the markdown body, taken literally")
	flags.StringVar(&options.bodyFile, flagBodyFile, "", "file holding the markdown body; - reads stdin")
	flags.StringVar(&options.why, flagWhy, "", "text of a \"## Why\" section added after the body")
	flags.StringArrayVar(&options.acceptance, flagAcceptance, nil,
		"a bullet of the \"## Acceptance\" section added last; repeat for more")
	flags.StringSliceVar(&options.deps, flagDep, nil, "issues the new one depends on; repeat or separate with commas")
	a.completeFlag(cmd, flagDep, commaSeparated(completeIDValue(issueIDs)))
	flags.BoolVar(&options.isStanding, flagStanding, false, "mark the issue standing: it only groups its children")
	_ = cmd.MarkFlagRequired(flagID)    // fails only for an undefined flag
	_ = cmd.MarkFlagRequired(flagTitle) // fails only for an undefined flag
	cmd.MarkFlagsMutuallyExclusive(flagBody, flagBodyFile)
	return a.withKV(cmd)
}

// create writes the new issue the flags describe through ops.CreateIssue, with a warning for every
// tree problem that concerns neither the issue's file nor its parent's, unless the index refresh
// already warned about it, then a warning when the new issue has no body or only whitespace, then
// one when its --plan is in none of the plans directories. A --dep that resolves to no issue fails
// the create before anything is written.
func (a *app) create(cmd *cobra.Command, options *createOptions) (output.Result, error) {
	created, err := a.ops().CreateIssue(newIssue(cmd, options))
	if err != nil {
		return output.Result{}, a.createFailure(err, options.id)
	}
	a.noteKindWrite(issueIDs, created.ID)
	result := writeResult(created.ID, created.Path, created.ETag)
	result.Warnings = append(result.Warnings, a.unwarnedTreeProblems(created.Problems)...)
	a.warnRaisedFormat(created.Effects)
	if created.IsBodyEmpty {
		result.Warnings = append(result.Warnings, fmt.Sprintf(emptyBodyWarning, created.ID, created.ID))
	}
	result.Warnings = append(result.Warnings, missingPlanWarnings(created.Path, created.MissingPlan)...)
	return result, nil
}

// newIssue returns the ops.NewIssue the flags describe, created now. --body-file is read only when
// ops.CreateIssue builds the issue, after every check has passed.
func newIssue(cmd *cobra.Command, options *createOptions) ops.NewIssue {
	spec := ops.NewIssue{
		ID:         options.id,
		Parent:     options.parent,
		Title:      options.title,
		Status:     options.status,
		Type:       options.issueType,
		Priority:   &options.priority,
		Labels:     options.labels,
		Plan:       options.plan,
		Deps:       options.deps,
		IsStanding: options.isStanding,
		Why:        options.why,
		Acceptance: options.acceptance,
		Created:    time.Now().UTC().Truncate(time.Second),
	}
	switch {
	case options.bodyFile != "":
		spec.Body = func() (string, error) { return readTextInput(cmd, flagBodyFile, options.bodyFile) }
	case options.body != "":
		spec.Body = func() (string, error) { return options.body, nil }
	}
	return spec
}

// createFailure words a refusal of ops.CreateIssue: a value the flags do not accept, a closed
// status, with the commands that create and then close the issue, a standing mark its status
// forbids, a --dep naming no issue, and every failure of the node store's create.
func (a *app) createFailure(err error, id string) error {
	var value *ops.ValueError
	if errors.As(err, &value) {
		return valueFailure(err, invalidErrorf)
	}
	var status *issue.StatusError
	if errors.As(err, &status) && errors.Is(err, ops.ErrCreatedClosed) {
		return &output.Error{
			Code: output.CodeInvalid,
			Message: fmt.Sprintf(
				"status %q is in the %s category; an issue is created open and closed after",
				status.Status,
				status.Category,
			),
			Hint: fmt.Sprintf(
				"capcom create --%s %s --%s <title>, then capcom close %s --%s <text>",
				flagID, id, flagTitle, id, flagReason,
			),
		}
	}
	if status != nil {
		return statusChangeFailure(id, err)
	}
	var refusal *reflist.Refusal
	if errors.As(err, &refusal) {
		return a.refusalFailure(err, issueIDs, "", refusalWords{})
	}
	return a.kindFailure(err, issueIDs)
}

// missingPlanWarnings returns the warning of a write that set a plan on the issue file at issuePath
// while the plan is in none of the plans directories, "<issuePath>: <message>" with the message
// capcom check reports for it (ops.Ops.MissingPlan); none when message is empty.
func missingPlanWarnings(issuePath string, message string) []string {
	if message == "" {
		return nil
	}
	return []string{issuePath + ": " + message}
}
