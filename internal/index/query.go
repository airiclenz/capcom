package index

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/tree"
)

// PlanState selects issues by whether their plan field is set. The index never reads the plans
// directories, so a set field that resolves to no file still counts as set; whether an issue is
// Planned is briefing's to decide.
type PlanState int

// The plan states a Filter can ask for.
const (
	// PlanAny keeps issues with and without a plan.
	PlanAny PlanState = iota
	// PlanSet keeps only issues whose plan field is set.
	PlanSet
	// PlanUnset keeps only issues whose plan field is not set.
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
// right before its children. Every row comes from one read snapshot.
func (x *Index) List(filter Filter) (entries []Entry, err error) {
	err = x.snapshot(func(q querier) error {
		ids, err := x.listIDs(q, filter)
		if err != nil {
			return err
		}
		entries, err = x.issues(q, ids)
		return err
	})
	return entries, err
}

// ListIDs returns the ids of the issues List returns for filter, in the same order, without
// reading the issues themselves.
func (x *Index) ListIDs(filter Filter) ([]string, error) {
	return x.listIDs(x.db, filter)
}

// listIDs reads the ids ListIDs returns through q.
func (x *Index) listIDs(q querier, filter Filter) ([]string, error) {
	query, args := filter.sql()
	return x.strings(context.Background(), q, query, args...)
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
		closed := config.ClosedCategories()
		conditions = append(conditions, "category NOT IN ("+placeholders(len(closed))+")")
		for _, category := range closed {
			args = append(args, string(category))
		}
	}

	query := "SELECT id FROM issues WHERE " + strings.Join(conditions, " AND ") + " ORDER BY path"
	if f.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, f.Limit)
	}
	return query, args
}

// Resolve finds the usable issue an argument names (ADR 0002), as tree.ResolveIn finds it among
// the names the latest refresh's membership verdicts stored: an exact id first, then an Alias, then
// the one id the name is a prefix of. It fails with an error matching tree.ErrNotFound when nothing
// matches, and with a *tree.AmbiguousError when the prefix matches several ids: every file's id
// counts, a file with problems or one left out for a repeated id included. A name two files claim,
// or that stands for a repeated id, fails with a *tree.Error wrapping tree.ErrDuplicate, and a name
// only an unusable file claims with a *tree.UnusableError naming that file, never as a prefix. An
// issue file with problems is no usable issue: its exact id fails as an unusable claim too. Every
// row comes from one read snapshot.
func (x *Index) Resolve(arg string) (entry Entry, err error) {
	err = x.snapshot(func(q querier) error {
		id, err := x.resolveID(q, issueNodes, arg)
		if err != nil {
			return err
		}
		entry, err = x.resolved(q, arg, id)
		return err
	})
	return entry, err
}

// resolveID finds the id of the usable node of one kind that arg names, through tree.ResolveIn
// over the kind's names as q reads them. Its errors quote arg as given, not its normalised name.
func (x *Index) resolveID(q querier, nodes nodeTable, arg string) (string, error) {
	return tree.ResolveIn(snapshotNames{x: x, q: q, nodes: nodes}, arg)
}

// IssueNames returns every id and Alias of a usable issue, each once and in order: the names an
// argument could have meant, from one read snapshot.
func (x *Index) IssueNames() ([]string, error) {
	return x.names(issueNodes)
}

// NoteNames returns every id and Alias of a usable Note, each once and in order, as IssueNames
// does for issues.
func (x *Index) NoteNames() ([]string, error) {
	return x.names(noteNodes)
}

// names returns every id and Alias of the usable nodes of one kind, each once and in order.
func (x *Index) names(nodes nodeTable) (names []string, err error) {
	table := nodes.table
	err = x.snapshot(func(q querier) error {
		names, err = x.strings(context.Background(), q, `SELECT id FROM `+table+` WHERE excluded = 0
			UNION SELECT aliases.alias FROM aliases JOIN `+table+` ON `+table+`.path = aliases.path
			WHERE aliases.kind = ? AND `+table+`.excluded = 0 ORDER BY 1`, nodes.kind)
		return err
	})
	return names, err
}

// snapshotNames is the tree.NameReader of one kind as a read snapshot of the index holds them: the
// stored membership verdicts and the usable node rows.
type snapshotNames struct {
	x     *Index
	q     querier
	nodes nodeTable
}

// Match returns the stored refusal of name among the names of the kind, and the usable node whose
// id or alias it is.
func (n snapshotNames) Match(name string) (tree.NameMatch, error) {
	ctx := context.Background()
	match, err := n.refusal(ctx, name)
	if err != nil {
		return tree.NameMatch{}, err
	}
	table := n.nodes.table
	exact, err := n.x.strings(ctx, n.q, `SELECT id FROM `+table+` WHERE id = ? AND excluded = 0`, name)
	if err != nil || len(exact) == 1 {
		match.ID = firstOf(exact)
		return match, err
	}
	owners, err := n.x.strings(ctx, n.q, `SELECT `+table+`.id FROM aliases
		JOIN `+table+` ON `+table+`.path = aliases.path
		WHERE aliases.kind = ? AND aliases.alias = ? AND `+table+`.excluded = 0`, n.nodes.kind, name)
	if err != nil || len(owners) == 1 {
		match.ID, match.IsAlias = firstOf(owners), true
	}
	return match, err
}

// PrefixCandidates returns the ids of the kind's walk files that name is a prefix of, from the
// stored file verdicts.
func (n snapshotNames) PrefixCandidates(name string) ([]string, error) {
	if name == "" {
		return nil, nil
	}
	return n.x.strings(context.Background(), n.q, `SELECT DISTINCT id FROM file_verdicts
		WHERE kind = ?1 AND substr(id, 1, length(?2)) = ?2 ORDER BY id`, n.nodes.kind, name)
}

// refusal returns a match carrying the stored refusal of name among the names of the kind and the
// conflict behind it, or no refusal when nothing refuses it.
func (n snapshotNames) refusal(ctx context.Context, name string) (tree.NameMatch, error) {
	match := tree.NameMatch{Conflict: tree.Conflict{Name: name}}
	var reason string
	err := n.q.QueryRowContext(ctx, `SELECT refusal, path, other, is_alias, is_other_alias
		FROM name_verdicts WHERE kind = ? AND name = ?`, n.nodes.kind, name).Scan(
		&reason, &match.Conflict.Key, &match.Conflict.Other,
		&match.Conflict.IsAlias, &match.Conflict.IsOtherAlias,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return tree.NameMatch{}, nil
	}
	if err != nil {
		return tree.NameMatch{}, fmt.Errorf("%s: query: %w", Path(n.x.store), err)
	}
	for refusal, stored := range refusalNames {
		if stored == reason {
			match.Refusal = refusal
			return match, nil
		}
	}
	return tree.NameMatch{}, fmt.Errorf("%s: unknown refusal %q of %s %q", Path(n.x.store), reason, n.nodes.kind, name)
}

// Node returns the issue as the ready rules and the Dependency graph see it.
func (e Entry) Node() issue.Node {
	return issue.Node{
		ID:         e.ID,
		ParentID:   e.ParentID,
		Category:   e.Category,
		IsStanding: e.IsStanding,
		Labels:     e.Labels,
		Deps:       e.Deps,
		Aliases:    e.Aliases,
	}
}

// Nodes returns the Node of each of entries, in their order.
func Nodes(entries []Entry) []issue.Node {
	nodes := make([]issue.Node, 0, len(entries))
	for _, entry := range entries {
		nodes = append(nodes, entry.Node())
	}
	return nodes
}

// firstOf returns the first value, or "" when there is none.
func firstOf(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// resolved reads the issue arg resolved to through q; it is gone only if the index changed
// underneath.
func (x *Index) resolved(q querier, arg, id string) (Entry, error) {
	entry, found, err := x.readIssue(q, id)
	if err == nil && !found {
		err = fmt.Errorf("id %q %w", arg, tree.ErrNotFound)
	}
	return entry, err
}

// SearchIssues returns the usable issues Search finds for text, best match first. Every row comes
// from one read snapshot.
func (x *Index) SearchIssues(text string) ([]Entry, error) {
	return x.searchIssues(text, matchExpression)
}

// SearchIssuesAnyWord returns the usable issues whose title or body contains at least one word
// of text, ranked by FTS5 rank then id. Words match as Search's do, FTS5 operators included.
// Every row comes from one read snapshot.
func (x *Index) SearchIssuesAnyWord(text string) ([]Entry, error) {
	return x.searchIssues(text, anyMatchExpression)
}

// searchIssues reads, from one snapshot, the usable issues searchIDs finds for text through
// expression.
func (x *Index) searchIssues(text string, expression func(words []string) string) (entries []Entry, err error) {
	err = x.snapshot(func(q querier) error {
		ids, err := x.searchIDs(q, text, expression)
		if err != nil {
			return err
		}
		entries, err = x.issues(q, ids)
		return err
	})
	return entries, err
}

// beforeEntryRead runs once before issues reads its entries, with the first id, and before noteList
// reads each entry; tests swap it to change the index between the read of the ids and the read of
// the entries.
var beforeEntryRead = func(id string) {}

// issues reads the indexed issue of each id through q, in order, in a fixed number of queries
// however many ids there are. An id no usable issue has, gone only if the index changed
// underneath, fails with an error matching tree.ErrNotFound.
func (x *Index) issues(q querier, ids []string) ([]Entry, error) {
	entries := make([]Entry, 0, len(ids))
	if len(ids) == 0 {
		return entries, nil
	}
	beforeEntryRead(ids[0])
	byID, err := x.loadIssues(q, ids)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		entry, found := byID[id]
		if !found {
			return nil, fmt.Errorf("id %q %w", id, tree.ErrNotFound)
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
