package cli

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/reflist"
)

// Flags of capcom create.
const (
	flagDep        = "dep"
	flagBody       = "body"
	flagWhy        = "why"
	flagAcceptance = "acceptance"
)

// The headings of the body sections create builds from --why and --acceptance.
const (
	whyHeading        = "## Why"
	acceptanceHeading = "## Acceptance"
)

// preferredDefaultType is the type a new issue gets without --type, when the config declares it;
// otherwise it gets the first declared type.
const preferredDefaultType = "task"

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

// create places the new issue in the tree and writes it, with a warning for every tree problem
// that concerns neither the issue's file nor its parent's, unless the index refresh already
// warned about it, and a warning when the new issue has no body or only whitespace. A --dep that
// resolves to no issue fails the create before anything is written.
func (a *app) create(cmd *cobra.Command, options *createOptions) (output.Result, error) {
	if err := a.checkCreateValues(options); err != nil {
		return output.Result{}, err
	}
	deps, err := a.resolveDeps(options.deps)
	if err != nil {
		return output.Result{}, err
	}
	var body string
	result, err := createNode(
		a,
		issueVerbs,
		options.id,
		options.parent,
		func() (issue.Issue, error) {
			created, err := a.newIssue(cmd, options, deps)
			body = created.Body
			return created, err
		},
	)
	if err != nil {
		return output.Result{}, err
	}
	if strings.TrimSpace(body) == "" {
		result.Warnings = append(result.Warnings, fmt.Sprintf(emptyBodyWarning, options.id, options.id))
	}
	return result, nil
}

// checkCreateValues refuses an undeclared --status or --type and a --priority out of range, listing
// the values the flag accepts, before anything is resolved or written. An empty --status or --type
// takes the default, so it is not checked.
func (a *app) checkCreateValues(options *createOptions) error {
	values := invalidValues(a.store.Config)
	if options.status != "" {
		if err := values.status(options.status); err != nil {
			return err
		}
	}
	if options.issueType != "" {
		if err := values.issueType(options.issueType); err != nil {
			return err
		}
	}
	return values.priority(options.priority)
}

// resolveDeps returns the canonical ids of the issues args name, in the order given and once each;
// an empty arg is skipped. No cycle is possible, since nothing depends on a new issue yet.
func (a *app) resolveDeps(args []string) ([]string, error) {
	var deps []string
	for _, arg := range args {
		if arg == "" {
			continue
		}
		addition, err := reflist.List{Entries: deps, Resolve: a.ops().ResolveIssueID, Names: namesItself}.Add(arg)
		if err != nil {
			return nil, a.refusalFailure(err, issueIDs, "", refusalWords{})
		}
		if !addition.IsPresent {
			deps = append(deps, addition.ID)
		}
	}
	return deps, nil
}

// namesItself is the reflist.NamesFunc of a new issue's deps, which resolveDeps writes as
// canonical ids only: each entry names the issue whose id it is.
func namesItself(entry string) (string, bool) {
	return entry, true
}

// newIssue builds the issue the flags describe, created now, depending on deps. With --standing it
// is marked standing once its status is chosen, so a status in the doing category is refused.
func (a *app) newIssue(cmd *cobra.Command, options *createOptions, deps []string) (issue.Issue, error) {
	cfg := a.store.Config
	status := options.status
	if status == "" {
		status, _ = cfg.DefaultStatus(config.CategoryTodo) // config guarantees a todo status
	}
	if category, _ := cfg.CategoryOf(status); category.IsClosed() {
		return issue.Issue{}, &output.Error{
			Code:    output.CodeInvalid,
			Message: fmt.Sprintf("status %q is in the %s category; an issue is created open and closed after", status, category),
			Hint: fmt.Sprintf(
				"capcom create --%s %s --%s <title>, then capcom close %s --%s <text>",
				flagID, options.id, flagTitle, options.id, flagReason,
			),
		}
	}
	issueType := options.issueType
	if issueType == "" {
		issueType = defaultType(cfg)
	}
	created := issue.Issue{
		Title:    options.title,
		Status:   status,
		Type:     issueType,
		Priority: options.priority,
		Labels:   edit.List(nil, options.labels, nil),
		Plan:     options.plan,
		Deps:     deps,
		Created:  time.Now().UTC().Truncate(time.Second),
	}
	if options.isStanding {
		marked, err := issue.SetStanding(created, cfg, true)
		if err != nil {
			return issue.Issue{}, statusChangeFailure(options.id, err)
		}
		created = marked
	}
	body, err := newIssueBody(cmd, options)
	if err != nil {
		return issue.Issue{}, err
	}
	created.Body = body
	return created, nil
}

// newIssueBody returns the body the flags describe: the --body or --body-file text, then a
// "## Why" section holding --why, then an "## Acceptance" section with one bullet per non-empty
// --acceptance, each block separated by a blank line. A flag left out adds nothing, so no flag
// gives an empty body.
func newIssueBody(cmd *cobra.Command, options *createOptions) (string, error) {
	text := options.body
	if options.bodyFile != "" {
		read, err := readTextInput(cmd, flagBodyFile, options.bodyFile)
		if err != nil {
			return "", err
		}
		text = read
	}
	body := ""
	if options.body != "" || options.bodyFile != "" {
		body = edit.AppendBody(body, text)
	}
	if options.why != "" {
		body = edit.AppendBody(body, whyHeading+"\n\n"+options.why)
	}
	var bullets []string
	for _, criterion := range options.acceptance {
		if criterion != "" {
			bullets = append(bullets, "- "+criterion)
		}
	}
	if len(bullets) > 0 {
		body = edit.AppendBody(body, acceptanceHeading+"\n\n"+strings.Join(bullets, "\n"))
	}
	return body, nil
}

// defaultType is the type of an issue created without --type; config guarantees at least one
// declared type.
func defaultType(cfg config.Config) string {
	if slices.Contains(cfg.Types, preferredDefaultType) {
		return preferredDefaultType
	}
	return cfg.Types[0]
}
