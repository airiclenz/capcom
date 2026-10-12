package cli

import (
	"slices"
	"time"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/nodestore"
)

// Flags of capcom create; --title, --status, --type, --priority, --label, --parent and --plan are
// shared with list and update.
const (
	flagID       = "id"
	flagBodyFile = "body-file"
)

// preferredDefaultType is the type a new issue gets without --type, when the config declares it;
// otherwise it gets the first declared type.
const preferredDefaultType = "task"

// createOptions holds the flags of one capcom create.
type createOptions struct {
	id        string
	title     string
	parent    string
	issueType string
	priority  int
	labels    []string
	plan      string
	status    string
	bodyFile  string
}

// newCreateCommand writes a new issue file. The id is always the caller's choice, never generated.
func (a *app) newCreateCommand() *cobra.Command {
	options := &createOptions{}
	cmd := &cobra.Command{
		Use:         "create --id <slug> --title <title>",
		Short:       "Create an issue with a chosen id",
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
	flags.StringVar(&options.id, flagID, "", "the new issue's id: a kebab slug of 2 to 4 words (required)")
	flags.StringVar(&options.title, flagTitle, "", "the issue's title (required)")
	flags.StringVar(&options.parent, flagParent, "", "create the issue below this one")
	flags.StringVar(&options.issueType, flagType, "", "the issue's type (default task, or the first configured type)")
	flags.IntVar(&options.priority, flagPriority, issue.DefaultPriority, "priority, 0 (most urgent) to 4")
	flags.StringSliceVar(&options.labels, flagLabel, nil, "labels; repeat or separate with commas")
	flags.StringVar(&options.plan, flagPlan, "", "basename of the plan the issue belongs to")
	flags.StringVar(&options.status, flagStatus, "", "initial status, not in a closed category (default: the first todo status)")
	flags.StringVar(&options.bodyFile, flagBodyFile, "", "file holding the markdown body; - reads stdin")
	_ = cmd.MarkFlagRequired(flagID)    // fails only for an undefined flag
	_ = cmd.MarkFlagRequired(flagTitle) // fails only for an undefined flag
	return a.withKV(cmd)
}

// create places the new issue in the tree and writes it, with a warning for every tree problem
// that concerns neither the issue's file nor its parent's, unless the index refresh already
// warned about it.
func (a *app) create(cmd *cobra.Command, options *createOptions) (Result, error) {
	written, problems, err := nodestore.Issues(a.store, nil).Create(
		options.id,
		options.parent,
		func() (issue.Issue, error) { return a.newIssue(cmd, options) },
	)
	if err != nil {
		return Result{}, kindFailure(err)
	}
	result := writeResult(written.ID, written.Path, written.ETag)
	result.Warnings = append(result.Warnings, a.unwarnedTreeProblems(problems)...)
	return result, nil
}

// newIssue builds the issue the flags describe, created now.
func (a *app) newIssue(cmd *cobra.Command, options *createOptions) (issue.Issue, error) {
	cfg := a.store.Config
	status := options.status
	if status == "" {
		status, _ = cfg.DefaultStatus(config.CategoryTodo) // config guarantees a todo status
	}
	if category, _ := cfg.CategoryOf(status); isClosedCategory(category) {
		return issue.Issue{}, Errorf(CodeInvalid, "status %q is in the %s category; create the issue open, then close it", status, category)
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
		Labels:   edit.Labels(nil, options.labels, nil),
		Plan:     options.plan,
		Created:  time.Now().UTC().Truncate(time.Second),
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
