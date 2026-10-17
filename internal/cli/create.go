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
	flagDep = "dep"
)

// preferredDefaultType is the type a new issue gets without --type, when the config declares it;
// otherwise it gets the first declared type.
const preferredDefaultType = "task"

// emptyBodyWarning is the warning of a create that leaves the new issue without a body; it takes
// the new issue's id twice.
const emptyBodyWarning = `%s has no body: state what, why and a "## Acceptance" section with capcom update %s --body-append-file <file>`

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
	bodyFile   string
	deps       []string
	isStanding bool
}

// The help text of capcom create.
const (
	createLong = "Create an Issue under the id you choose: a kebab-case slug of 1 to 4 words that names the " +
		"work. --parent files it below another Issue, and --dep makes it wait for the Issues named; a " +
		"--dep that names no Issue fails the create before anything is written. The body comes from " +
		"--body-file (- reads stdin) and states what is wrong or wanted, why, and a \"## Acceptance\" " +
		"section; an Issue created without a body draws a warning. --standing marks it a standing " +
		"Parent that only groups its children: it is never Ready, and it is not claimed or closed while " +
		"marked, so it cannot be created in the doing category. The first standing Issue in a store " +
		"below format 5 raises the store format to 5, with a warning that older capcom releases can no " +
		"longer read the store. It prints the new Issue's id, path and etag."
	createExample = "  capcom create --id login-timeout-race --title \"Session refresh races the login timeout\" --type bug\n" +
		"  capcom create --id refresh-retry --title \"Retry a failed token refresh\" --dep login-timeout-race   # waits for it\n" +
		"  capcom create --id login-copy --title \"Reword the login errors\" --parent login-flow --priority 1\n" +
		"  capcom create --id cache-warmup --title \"Warm the cache on deploy\" --body-file - < body.md   # body from stdin\n" +
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
	flags.StringVar(&options.issueType, flagType, "", "the issue's type (default task, or the first configured type)")
	flags.IntVar(&options.priority, flagPriority, issue.DefaultPriority, "priority, 0 (most urgent) to 4")
	flags.StringSliceVar(&options.labels, flagLabel, nil, "labels; repeat or separate with commas")
	flags.StringVar(&options.plan, flagPlan, "", "basename of the plan the issue belongs to")
	flags.StringVar(&options.status, flagStatus, "", "initial status, not in a closed category (default: the first todo status)")
	flags.StringVar(&options.bodyFile, flagBodyFile, "", "file holding the markdown body; - reads stdin")
	flags.StringSliceVar(&options.deps, flagDep, nil, "issues the new one depends on; repeat or separate with commas")
	flags.BoolVar(&options.isStanding, flagStanding, false, "mark the issue standing: it only groups its children")
	_ = cmd.MarkFlagRequired(flagID)    // fails only for an undefined flag
	_ = cmd.MarkFlagRequired(flagTitle) // fails only for an undefined flag
	return a.withKV(cmd)
}

// create places the new issue in the tree and writes it, with a warning for every tree problem
// that concerns neither the issue's file nor its parent's, unless the index refresh already
// warned about it, and a warning when the new issue has no body or only whitespace. A --dep that
// resolves to no issue fails the create before anything is written.
func (a *app) create(cmd *cobra.Command, options *createOptions) (output.Result, error) {
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
			return nil, refusalFailure(err, "", refusalWords{})
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
		return issue.Issue{}, output.Errorf(output.CodeInvalid, "status %q is in the %s category; create the issue open, then close it", status, category)
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
	if options.bodyFile == "" {
		return created, nil
	}
	body, err := readTextInput(cmd, options.bodyFile)
	if err != nil {
		return issue.Issue{}, err
	}
	created.Body = edit.AppendBody(created.Body, body)
	return created, nil
}

// defaultType is the type of an issue created without --type; config guarantees at least one
// declared type.
func defaultType(cfg config.Config) string {
	if slices.Contains(cfg.Types, preferredDefaultType) {
		return preferredDefaultType
	}
	return cfg.Types[0]
}
