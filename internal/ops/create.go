package ops

import (
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/nodestore"
	"github.com/airiclenz/capcom/internal/reflist"
	"github.com/airiclenz/capcom/internal/tree"
)

// The headings of the body sections CreateIssue builds from NewIssue.Why and NewIssue.Acceptance.
const (
	whyHeading        = "## Why"
	acceptanceHeading = "## Acceptance"
)

// preferredDefaultType is the type a new issue gets without one, when the config declares it;
// otherwise it gets the first declared type.
const preferredDefaultType = "task"

// ErrCreatedClosed refuses a create whose status is in a closed category: an issue is created open
// and closed after. It comes wrapped in an *issue.StatusError naming the status and its category.
var ErrCreatedClosed = errors.New("an issue is created open and closed after")

// NewIssue describes the issue CreateIssue writes.
type NewIssue struct {
	// ID is the new issue's id, the caller's choice.
	ID string
	// Parent names the issue to create it below; "" places it at the top of the tree.
	Parent string
	// Title is the issue's title.
	Title string
	// Status is its status, outside the closed categories; "" takes the first todo status.
	Status string
	// Type is its type; "" takes task when config.yaml declares it, else the first declared type.
	Type string
	// Priority is its priority; nil takes issue.DefaultPriority.
	Priority *int
	// Labels are its labels, kept once each.
	Labels []string
	// Plan is the basename of the plan it belongs to; "" sets none.
	Plan string
	// Deps reference the issues it depends on; an empty entry is skipped.
	Deps []string
	// IsStanding marks it standing, so it only groups its children.
	IsStanding bool
	// Body returns the text the body starts with; nil gives none. It runs only once every check
	// has passed, just before the file is encoded, so a body read from a file or stdin is read last.
	Body func() (string, error)
	// Why is the text of a "## Why" section added after the body; "" adds none.
	Why string
	// Acceptance holds the bullets of the "## Acceptance" section added last; empty ones are skipped.
	Acceptance []string
	// Created is the issue's created time, stored as given.
	Created time.Time
}

// CreateResult is the outcome of one create: the issue written, with the Effects of its write,
// the tree problems that do not concern it, for the caller to report as warnings, whether its body
// is empty or only white space, and the message capcom check reports for its plan when the plan is
// in none of the plans directories ("" otherwise).
type CreateResult struct {
	nodestore.Written
	Problems    tree.Errors
	IsBodyEmpty bool
	MissingPlan string
}

// CreateIssue writes the new issue spec describes through the issue tree's node store
// (nodestore.Kind.Create), checking, in this order and stopping at the first failure: the plan,
// status, type and priority values (each refusal a *ValueError); the deps, each resolved through
// the Catalog to its canonical id, once each, in the order given (a dep naming no issue, the new
// issue itself included, is the *reflist.Refusal of reflist.List.Add); then every check of
// Kind.Create, with the issue built at its build step. The build refuses a status in a closed
// category as an *issue.StatusError wrapping ErrCreatedClosed and a standing mark its status
// forbids as issue.SetStanding's *issue.StatusError, before spec.Body runs. Every refusal returns
// unwrapped, with no path; no cycle is possible, since nothing depends on a new issue yet.
func (o *Ops) CreateIssue(spec NewIssue) (CreateResult, error) {
	if err := checkNewIssue(o.store.Config, spec); err != nil {
		return CreateResult{}, err
	}
	deps, err := o.resolveNewDeps(spec.Deps)
	if err != nil {
		return CreateResult{}, err
	}
	var body string
	written, problems, err := o.issues().Create(spec.ID, spec.Parent, func() (issue.Issue, error) {
		created, err := buildIssue(o.store.Config, spec, deps)
		body = created.Body
		return created, err
	})
	if err != nil {
		return CreateResult{}, err
	}
	return CreateResult{
		Written:     written,
		Problems:    problems,
		IsBodyEmpty: strings.TrimSpace(body) == "",
		MissingPlan: o.MissingPlan(spec.Plan),
	}, nil
}

// MissingPlan returns the message capcom check reports for an issue whose plan is in none of the
// plans directories, "" when plan is empty or resolves to a file.
func (o *Ops) MissingPlan(plan string) string {
	if plan == "" {
		return ""
	}
	if _, isFound := o.store.Config.PlanPath(o.store.WorktreeRoot, plan); isFound {
		return ""
	}
	return o.store.Config.MissingPlanMessage(plan)
}

// checkNewIssue refuses, as a *ValueError, a plan that is not a bare basename, an undeclared status
// or type and a priority out of range, in that order. An empty status or type takes the default,
// so it is not checked.
func checkNewIssue(cfg config.Config, spec NewIssue) error {
	if err := CheckPlan(cfg, spec.Plan); err != nil {
		return err
	}
	if spec.Status != "" {
		if err := CheckStatus(cfg, spec.Status); err != nil {
			return err
		}
	}
	if spec.Type != "" {
		if err := CheckType(cfg, spec.Type); err != nil {
			return err
		}
	}
	if spec.Priority != nil {
		return CheckPriority(*spec.Priority)
	}
	return nil
}

// resolveNewDeps returns the canonical ids of the issues refs name, in the order given and once
// each; an empty ref is skipped.
func (o *Ops) resolveNewDeps(refs []string) ([]string, error) {
	var deps []string
	for _, ref := range refs {
		if ref == "" {
			continue
		}
		addition, err := reflist.List{Entries: deps, Resolve: o.ResolveIssueID, Names: namesItself}.Add(ref)
		if err != nil {
			return nil, err
		}
		if !addition.IsPresent {
			deps = append(deps, addition.ID)
		}
	}
	return deps, nil
}

// namesItself is the reflist.NamesFunc of a new issue's deps, which resolveNewDeps writes as
// canonical ids only: each entry names the issue whose id it is.
func namesItself(entry string) (string, bool) {
	return entry, true
}

// buildIssue builds the issue spec describes, depending on deps. A standing mark is set once the
// status is chosen, so a status in the doing category is refused; spec.Body runs last.
func buildIssue(cfg config.Config, spec NewIssue, deps []string) (issue.Issue, error) {
	status := spec.Status
	if status == "" {
		status, _ = cfg.DefaultStatus(config.CategoryTodo) // config guarantees a todo status
	}
	if category, _ := cfg.CategoryOf(status); category.IsClosed() {
		return issue.Issue{}, &issue.StatusError{Status: status, Category: category, Err: ErrCreatedClosed}
	}
	issueType := spec.Type
	if issueType == "" {
		issueType = defaultType(cfg)
	}
	priority := issue.DefaultPriority
	if spec.Priority != nil {
		priority = *spec.Priority
	}
	created := issue.Issue{
		Title:    spec.Title,
		Status:   status,
		Type:     issueType,
		Priority: priority,
		Labels:   edit.List(nil, spec.Labels, nil),
		Plan:     spec.Plan,
		Deps:     deps,
		Created:  spec.Created,
	}
	if spec.IsStanding {
		marked, err := issue.SetStanding(created, cfg, true)
		if err != nil {
			return issue.Issue{}, err
		}
		created = marked
	}
	text := ""
	isBodyGiven := spec.Body != nil
	if isBodyGiven {
		read, err := spec.Body()
		if err != nil {
			return issue.Issue{}, err
		}
		text = read
	}
	created.Body = newIssueBody(text, isBodyGiven, spec.Why, spec.Acceptance)
	return created, nil
}

// newIssueBody returns the body of a new issue: text when isTextGiven, then a "## Why" section
// holding why, then an "## Acceptance" section with one bullet per non-empty criterion, each block
// separated by a blank line. A part left out adds nothing, so none gives an empty body.
func newIssueBody(text string, isTextGiven bool, why string, acceptance []string) string {
	body := ""
	if isTextGiven {
		body = edit.AppendBody(body, text)
	}
	if why != "" {
		body = edit.AppendBody(body, whyHeading+"\n\n"+why)
	}
	var bullets []string
	for _, criterion := range acceptance {
		if criterion != "" {
			bullets = append(bullets, "- "+criterion)
		}
	}
	if len(bullets) > 0 {
		body = edit.AppendBody(body, acceptanceHeading+"\n\n"+strings.Join(bullets, "\n"))
	}
	return body
}

// defaultType is the type of an issue created without one; config guarantees at least one
// declared type.
func defaultType(cfg config.Config) string {
	if slices.Contains(cfg.Types, preferredDefaultType) {
		return preferredDefaultType
	}
	return cfg.Types[0]
}
