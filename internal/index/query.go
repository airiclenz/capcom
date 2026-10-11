package index

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/tree"
)

// PlanState selects issues by whether they are Planned (have a plan).
type PlanState int

// The plan states a Filter can ask for.
const (
	// PlanAny keeps issues with and without a plan.
	PlanAny PlanState = iota
	// PlanSet keeps only Planned issues.
	PlanSet
	// PlanUnset keeps only issues without a plan.
	PlanUnset
)

// Filter selects the issues List returns. Empty fields select everything; within one list field
// any value matches, except Labels, where the issue must carry every one.
type Filter struct {
	Statuses   []string
	Categories []config.Category
	// Labels must all be on the issue.
	Labels []string
	// NoLabels must all be absent from the issue.
	NoLabels   []string
	Types      []string
	Priorities []int
	// ParentID keeps only the direct children of that issue.
	ParentID string
	// PlanContains keeps only issues whose plan holds this text, case-sensitively.
	PlanContains string
	Plan         PlanState
	// IsClosedIncluded keeps Closed issues (category done or dropped), which are left out otherwise.
	IsClosedIncluded bool
	// Limit caps the number of issues returned; 0 returns all of them.
	Limit int
}

// List returns the usable issues filter selects, in tree order: by path, so every issue comes
// right before its children.
func (x *Index) List(filter Filter) ([]Entry, error) {
	query, args := filter.sql()
	ids, err := x.strings(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	return x.issues(ids)
}

// sql builds the query selecting the ids filter keeps, and its arguments.
func (f Filter) sql() (string, []any) {
	conditions := []string{"excluded = 0"}
	var args []any
	addIn := func(column string, values []any) {
		if len(values) == 0 {
			return
		}
		conditions = append(conditions, column+" IN ("+placeholders(len(values))+")")
		args = append(args, values...)
	}
	addIn("status", toAny(f.Statuses))
	categories := make([]string, 0, len(f.Categories))
	for _, category := range f.Categories {
		categories = append(categories, string(category))
	}
	addIn("category", toAny(categories))
	addIn("type", toAny(f.Types))
	addIn("priority", toAny(f.Priorities))

	for _, label := range f.Labels {
		conditions = append(conditions, "EXISTS (SELECT 1 FROM labels WHERE labels.path = issues.path AND label = ?)")
		args = append(args, label)
	}
	if len(f.NoLabels) > 0 {
		conditions = append(conditions, "NOT EXISTS (SELECT 1 FROM labels WHERE labels.path = issues.path AND label IN ("+
			placeholders(len(f.NoLabels))+"))")
		args = append(args, toAny(f.NoLabels)...)
	}
	if f.ParentID != "" {
		conditions = append(conditions, "parent = ?")
		args = append(args, f.ParentID)
	}
	if f.PlanContains != "" {
		conditions = append(conditions, "instr(plan, ?) > 0")
		args = append(args, f.PlanContains)
	}
	switch f.Plan {
	case PlanSet:
		conditions = append(conditions, "plan <> ''")
	case PlanUnset:
		conditions = append(conditions, "plan = ''")
	}
	if !f.IsClosedIncluded {
		conditions = append(conditions, "category NOT IN (?, ?)")
		args = append(args, string(config.CategoryDone), string(config.CategoryDropped))
	}

	query := "SELECT id FROM issues WHERE " + strings.Join(conditions, " AND ") + " ORDER BY path"
	if f.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, f.Limit)
	}
	return query, args
}

// Resolve finds the usable issue an argument names (ADR 0002): an exact id first, then an Alias,
// then the one id the argument is a prefix of. It fails with an error matching tree.ErrNotFound
// when nothing matches, and with a *tree.AmbiguousError when the prefix matches several ids.
func (x *Index) Resolve(arg string) (Entry, error) {
	ctx := context.Background()
	entry, found, err := x.Issue(arg)
	if err != nil || found {
		return entry, err
	}

	owners, err := x.strings(ctx, `SELECT issues.id FROM aliases JOIN issues ON issues.path = aliases.path
		WHERE aliases.alias = ? AND issues.excluded = 0`, arg)
	if err != nil {
		return Entry{}, err
	}
	if len(owners) == 1 {
		return x.resolved(arg, owners[0])
	}

	var candidates []string
	if arg != "" {
		candidates, err = x.strings(ctx, `SELECT id FROM issues
			WHERE excluded = 0 AND substr(id, 1, length(?1)) = ?1 ORDER BY id`, arg)
		if err != nil {
			return Entry{}, err
		}
	}
	switch len(candidates) {
	case 0:
		return Entry{}, fmt.Errorf("id %q %w", arg, tree.ErrNotFound)
	case 1:
		return x.resolved(arg, candidates[0])
	default:
		return Entry{}, &tree.AmbiguousError{Arg: arg, Candidates: candidates}
	}
}

// resolved returns the issue arg resolved to; it is gone only if the index changed underneath.
func (x *Index) resolved(arg, id string) (Entry, error) {
	entry, found, err := x.Issue(id)
	if err == nil && !found {
		err = fmt.Errorf("id %q %w", arg, tree.ErrNotFound)
	}
	return entry, err
}

// Body returns the markdown body of the issue file at path (an Entry's Path), byte for byte.
func (x *Index) Body(path string) (string, error) {
	var body string
	err := x.db.QueryRowContext(context.Background(), `SELECT body FROM issues_fts WHERE path = ?`, path).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("%s: %w", path, tree.ErrNotFound)
	}
	if err != nil {
		return "", fmt.Errorf("%s: read body of %s: %w", Path(x.store), path, err)
	}
	return body, nil
}

// SearchIssues returns the usable issues Search finds for text, best match first.
func (x *Index) SearchIssues(text string) ([]Entry, error) {
	ids, err := x.Search(text)
	if err != nil {
		return nil, err
	}
	return x.issues(ids)
}

// issues returns the indexed issue of each id, in order.
func (x *Index) issues(ids []string) ([]Entry, error) {
	entries := make([]Entry, 0, len(ids))
	for _, id := range ids {
		entry, err := x.resolved(id, id)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// placeholders returns count comma-separated SQL parameters.
func placeholders(count int) string {
	return strings.TrimSuffix(strings.Repeat("?, ", count), ", ")
}

// toAny converts values to query arguments.
func toAny[T any](values []T) []any {
	converted := make([]any, 0, len(values))
	for _, value := range values {
		converted = append(converted, value)
	}
	return converted
}
