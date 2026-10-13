package index

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/tree"
)

// nodeFileExtension ends the name of every node file: a node's file is its id plus this.
const nodeFileExtension = ".md"

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
	ids, err := x.ListIDs(filter)
	if err != nil {
		return nil, err
	}
	return x.issues(ids)
}

// ListIDs returns the ids of the issues List returns for filter, in the same order, without
// reading the issues themselves.
func (x *Index) ListIDs(filter Filter) ([]string, error) {
	query, args := filter.sql()
	return x.strings(context.Background(), query, args...)
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
		conditions = append(conditions, "EXISTS (SELECT 1 FROM labels WHERE labels.path = issues.path AND labels.kind = ? AND label = ?)")
		args = append(args, kindIssue, label)
	}
	if len(f.NoLabels) > 0 {
		conditions = append(conditions, "NOT EXISTS (SELECT 1 FROM labels WHERE labels.path = issues.path AND labels.kind = ? AND label IN ("+
			placeholders(len(f.NoLabels))+"))")
		args = append(args, kindIssue)
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

// Resolve finds the usable issue an argument names (ADR 0002), once tree.NormalizeArg has turned a
// citation spelling into a name: an exact id first, then an Alias, then the one id the name is a
// prefix of. It fails with an error matching tree.ErrNotFound
// when nothing matches, and with a *tree.AmbiguousError when the prefix matches several ids. A name
// only a node left out as a duplicate answers to, or the id or an alias of a node file under the
// directory of a repeated id, fails with a *tree.Error wrapping tree.ErrDuplicate, and the id or an
// alias of an invalid file, a file the walk leaves out unread or a node file under another
// directory the walk leaves out with tree.ErrNotFound, never as a prefix.
func (x *Index) Resolve(arg string) (Entry, error) {
	id, err := x.resolveID(issueNodes, arg)
	if err != nil {
		return Entry{}, err
	}
	return x.resolved(arg, id)
}

// resolveID finds the id of the usable node of one kind that arg names, the way Resolve does. Its
// errors quote arg as given, not its normalised name. A name no usable node has as its id or an
// alias, but that a node left out as a duplicate or a file the claims table records claims, fails
// as unusableClaim says and is never matched as a prefix. The ids of the node files the latest
// refresh left out below a repeated id count as prefix candidates, as tree.Resolve counts them: a
// prefix one of them shares with another id stays ambiguous, and a prefix of one of them alone
// fails with its duplicate problem.
func (x *Index) resolveID(nodes nodeTable, arg string) (string, error) {
	ctx := context.Background()
	name := tree.NormalizeArg(arg)
	exact, err := x.strings(ctx, `SELECT id FROM `+nodes.table+` WHERE id = ? AND excluded = 0`, name)
	if err != nil || len(exact) == 1 {
		return firstOf(exact), err
	}

	owners, err := x.strings(ctx, `SELECT `+nodes.table+`.id FROM aliases
		JOIN `+nodes.table+` ON `+nodes.table+`.path = aliases.path
		WHERE aliases.kind = ? AND aliases.alias = ? AND `+nodes.table+`.excluded = 0`, nodes.kind, name)
	if err != nil || len(owners) == 1 {
		return firstOf(owners), err
	}
	if err := x.unusableClaim(ctx, nodes, arg, name); err != nil {
		return "", err
	}

	var candidates []string
	if name != "" {
		candidates, err = x.strings(ctx, `SELECT id FROM `+nodes.table+`
			WHERE excluded = 0 AND substr(id, 1, length(?1)) = ?1 ORDER BY id`, name)
		if err != nil {
			return "", err
		}
		candidates = append(candidates, x.leftOutPrefixCandidates(nodes.kind, name)...)
		slices.Sort(candidates)
	}
	switch len(candidates) {
	case 0:
		return "", fmt.Errorf("id %q %w", arg, tree.ErrNotFound)
	case 1:
		if problem := x.leftOutProblem(nodes.kind, func(node leftOutNode) bool {
			return node.id == candidates[0]
		}); problem != nil {
			return "", problem
		}
		return candidates[0], nil
	default:
		return "", &tree.AmbiguousError{Arg: arg, Candidates: candidates}
	}
}

// unusableClaim returns why name, which no usable node of the kind answers to, still must not be
// matched as a prefix, or nil when nothing else claims it. A node left out as a duplicate that has
// name as its id or an alias fails with a *tree.Error wrapping tree.ErrDuplicate, naming its file
// and the other file of a name they share; a name a file of the kind with problems, a file the walk
// leaves out unread, or a node file under a directory the walk leaves out claims (its id or an
// alias, as the claims table records them) fails with an error wrapping tree.ErrNotFound
// that quotes arg and names the first such file in path order, unless that file is a node file the
// latest refresh left out below a repeated id: then it fails with that id's duplicate problem,
// naming both of its files.
func (x *Index) unusableClaim(ctx context.Context, nodes nodeTable, arg, name string) error {
	duplicate, err := x.duplicateClaim(ctx, nodes, name)
	if err != nil {
		return err
	}
	if duplicate != nil {
		return duplicate
	}

	invalid, err := x.strings(ctx, `SELECT path FROM claims
		WHERE kind = ? AND name = ? ORDER BY path LIMIT 1`, nodes.kind, name)
	if err != nil || len(invalid) == 0 {
		return err
	}
	if problem := x.leftOutProblem(nodes.kind, func(node leftOutNode) bool {
		return node.path == invalid[0]
	}); problem != nil {
		return problem
	}
	return fmt.Errorf("id %q %w: %s is invalid", arg, tree.ErrNotFound, invalid[0])
}

// leftOutProblem returns a copy of the duplicate problem of the first node file of the kind the
// latest refresh left out below a repeated id that isMatch accepts, or nil when it accepts none.
func (x *Index) leftOutProblem(kind string, isMatch func(leftOutNode) bool) *tree.Error {
	for _, node := range x.leftOut {
		if node.kind == kind && isMatch(node) {
			problem := *node.problem
			return &problem
		}
	}
	return nil
}

// leftOutPrefixCandidates returns the ids of the node files of the kind the latest refresh left
// out below a repeated id that name is a prefix of. Such an id is unique among the walk's node
// files, so it is never also the id of a row.
func (x *Index) leftOutPrefixCandidates(kind, name string) []string {
	var candidates []string
	for _, node := range x.leftOut {
		if node.kind == kind && strings.HasPrefix(node.id, name) {
			candidates = append(candidates, node.id)
		}
	}
	return candidates
}

// duplicateClaim returns the duplicate that left out a node of the kind having name as its id or
// an alias, or nil when no left-out node has. The problem is the one on name itself when two files
// claim it, otherwise one on another name the node shares with a different file; it is worded like
// the refresh warning, with both files relative to the working tree root.
func (x *Index) duplicateClaim(ctx context.Context, nodes nodeTable, name string) (*tree.Error, error) {
	rows, err := x.db.QueryContext(ctx, `WITH claims(name, path, kind) AS (
			SELECT id, path, 'id' FROM `+nodes.table+`
			UNION ALL SELECT alias, path, 'alias' FROM aliases WHERE kind = ?2
		)
		SELECT mine.path, mine.kind, mine.name, theirs.path, theirs.kind
		FROM claims AS mine
		JOIN claims AS theirs ON theirs.name = mine.name AND theirs.path <> mine.path
		WHERE mine.path IN (
			SELECT claims.path FROM claims JOIN `+nodes.table+` ON `+nodes.table+`.path = claims.path
			WHERE claims.name = ?1 AND `+nodes.table+`.excluded = 1
		)
		ORDER BY mine.name <> ?1, mine.path, mine.name, theirs.path
		LIMIT 1`, name, nodes.kind)
	if err != nil {
		return nil, fmt.Errorf("%s: query: %w", Path(x.store), err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("%s: query: %w", Path(x.store), err)
		}
		return nil, nil
	}
	var mine, theirs claim
	if err := rows.Scan(&mine.path, &mine.kind, &mine.name, &theirs.path, &theirs.kind); err != nil {
		return nil, fmt.Errorf("%s: query: %w", Path(x.store), err)
	}
	return &tree.Error{
		Path:  mine.path,
		Other: theirs.path,
		Err:   fmt.Errorf("%w %s %q: also the %s of", tree.ErrDuplicate, mine.kind, mine.name, theirs.kind),
	}, nil
}

// Node returns the issue as the ready rules and the Dependency graph see it.
func (e Entry) Node() issue.Node {
	return issue.Node{
		ID:       e.ID,
		ParentID: e.ParentID,
		Category: e.Category,
		Labels:   e.Labels,
		Deps:     e.Deps,
		Aliases:  e.Aliases,
	}
}

// firstOf returns the first value, or "" when there is none.
func firstOf(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// resolved returns the issue arg resolved to; it is gone only if the index changed underneath.
func (x *Index) resolved(arg, id string) (Entry, error) {
	entry, found, err := x.Issue(id)
	if err == nil && !found {
		err = fmt.Errorf("id %q %w", arg, tree.ErrNotFound)
	}
	return entry, err
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
