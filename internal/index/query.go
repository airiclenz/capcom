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

// Resolve finds the usable issue an argument names (ADR 0002), once tree.NormalizeArg has turned a
// citation spelling into a name: an exact id first, then an Alias, then the one id the name is a
// prefix of. It fails with an error matching tree.ErrNotFound when nothing matches, and with a
// *tree.AmbiguousError when the prefix matches several ids: every file's id counts, a file with
// problems or one left out for a repeated id included. A name only a node left out as a duplicate
// answers to, or the id or an alias of a node file under the directory of a repeated id, fails with
// a *tree.Error wrapping tree.ErrDuplicate, and so does a name a usable node has that an unusable
// file also claims. The id or an alias of an invalid file, a file the walk leaves out unread, a
// node file under another directory the walk leaves out, and an alias only the later file of a
// repeated id lists fail with tree.ErrNotFound, never as a prefix. Every row comes from one read
// snapshot.
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

// resolveID finds the id of the usable node of one kind that arg names, the way Resolve does, from
// the membership verdicts the latest refresh stored, as tree.Resolve finds it from its own. Its
// errors quote arg as given, not its normalised name. Before any lookup, a refused name fails as
// refusal says. A prefix counts the ids prefixCandidates gives: a prefix one unusable id shares
// with another id stays ambiguous, and a prefix of one unusable id alone fails as prefixMatch
// says. It reads through q.
func (x *Index) resolveID(q querier, nodes nodeTable, arg string) (string, error) {
	ctx := context.Background()
	name := tree.NormalizeArg(arg)
	refused, err := x.refusal(ctx, q, nodes.kind, name)
	if err != nil {
		return "", err
	}
	if refused != nil {
		return "", refused.err(arg)
	}
	exact, err := x.strings(ctx, q, `SELECT id FROM `+nodes.table+` WHERE id = ? AND excluded = 0`, name)
	if err != nil || len(exact) == 1 {
		return firstOf(exact), err
	}
	owners, err := x.strings(ctx, q, `SELECT `+nodes.table+`.id FROM aliases
		JOIN `+nodes.table+` ON `+nodes.table+`.path = aliases.path
		WHERE aliases.kind = ? AND aliases.alias = ? AND `+nodes.table+`.excluded = 0`, nodes.kind, name)
	if err != nil || len(owners) == 1 {
		return firstOf(owners), err
	}

	candidates, err := x.prefixCandidates(ctx, q, nodes.kind, name)
	if err != nil {
		return "", err
	}
	switch len(candidates) {
	case 0:
		return "", fmt.Errorf("id %q %w", arg, tree.ErrNotFound)
	case 1:
		return x.prefixMatch(ctx, q, nodes, arg, candidates[0])
	default:
		return "", &tree.AmbiguousError{Arg: arg, Candidates: candidates}
	}
}

// prefixCandidates returns the ids of the kind name is a prefix of, each once and in order, as
// tree.Resolve counts them: the id of every node file the walk found, kept or left out for a
// repeated id, with problems or without. A file the walk leaves out unread or under an orphan
// directory is no node of the tree and counts as none. An empty name has none. It reads through
// q.
func (x *Index) prefixCandidates(ctx context.Context, q querier, kind, name string) ([]string, error) {
	if name == "" {
		return nil, nil
	}
	return x.strings(ctx, q, `SELECT DISTINCT id FROM file_verdicts
		WHERE kind = ?1 AND substr(id, 1, length(?2)) = ?2 ORDER BY id`, kind, name)
}

// prefixMatch returns id, the one candidate the prefix arg matched, when a usable node of the kind
// has it, unless only an unusable file claims it. Otherwise it fails as the exact name id would,
// quoting arg where the refusal quotes the argument. It reads through q.
func (x *Index) prefixMatch(ctx context.Context, q querier, nodes nodeTable, arg, id string) (string, error) {
	refused, err := x.refusal(ctx, q, nodes.kind, id)
	if err != nil {
		return "", err
	}
	if refused != nil && refused.reason == refusalUnusable {
		return "", refused.err(arg)
	}
	usable, err := x.strings(ctx, q, `SELECT id FROM `+nodes.table+` WHERE id = ? AND excluded = 0`, id)
	if err != nil || len(usable) == 1 {
		return firstOf(usable), err
	}
	if refused != nil {
		return "", refused.err(arg)
	}
	return "", fmt.Errorf("id %q %w", arg, tree.ErrNotFound)
}

// storedRefusal is a name_verdicts row: why a name of one kind must not be resolved, and the
// conflict that refuses it.
type storedRefusal struct {
	reason   string
	conflict tree.Conflict
}

// refusal returns the stored refusal of name among the names of kind, or nil when nothing refuses
// it. It reads through q.
func (x *Index) refusal(ctx context.Context, q querier, kind, name string) (*storedRefusal, error) {
	refused := &storedRefusal{conflict: tree.Conflict{Name: name}}
	err := q.QueryRowContext(ctx, `SELECT refusal, path, other, is_alias, is_other_alias
		FROM name_verdicts WHERE kind = ? AND name = ?`, kind, name).Scan(
		&refused.reason, &refused.conflict.Key, &refused.conflict.Other,
		&refused.conflict.IsAlias, &refused.conflict.IsOtherAlias,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: query: %w", Path(x.store), err)
	}
	return refused, nil
}

// err returns the error the refusal fails a resolve of arg with, as tree.Resolve words it: a name
// only an unusable file claims fails with a *tree.UnusableError naming that file and quoting arg;
// any other with a *tree.Error wrapping tree.ErrDuplicate, the problem of its conflict naming both
// files, as a repeated id when the conflict's file claims the name as its id and as a duplicate
// alias otherwise.
func (s *storedRefusal) err(arg string) error {
	conflict := s.conflict
	switch {
	case s.reason == refusalUnusable:
		return &tree.UnusableError{Arg: arg, Path: conflict.Key}
	case !conflict.IsAlias:
		return tree.DuplicateIDError(conflict.Key, conflict.Other, conflict.Name)
	default:
		return tree.DuplicateAliasError(conflict.Key, conflict.Other, conflict.Name, conflict.IsOtherAlias)
	}
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
func (x *Index) SearchIssues(text string) (entries []Entry, err error) {
	err = x.snapshot(func(q querier) error {
		ids, err := x.searchIDs(q, text)
		if err != nil {
			return err
		}
		entries, err = x.issues(q, ids)
		return err
	})
	return entries, err
}

// beforeEntryRead runs before issues and noteList read each entry; tests swap it to change the
// index between the read of the ids and the read of an entry.
var beforeEntryRead = func(id string) {}

// issues reads the indexed issue of each id through q, in order.
func (x *Index) issues(q querier, ids []string) ([]Entry, error) {
	entries := make([]Entry, 0, len(ids))
	for _, id := range ids {
		beforeEntryRead(id)
		entry, err := x.resolved(q, id, id)
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
