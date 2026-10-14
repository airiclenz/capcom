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

// resolveID finds the id of the usable node of one kind that arg names, the way Resolve does. Its
// errors quote arg as given, not its normalised name. Before any lookup, a name two files claim
// fails as refusedClaim says, as tree.Resolve refuses it. A name no usable node has as its id or an
// alias, but that an unusable file claims, fails as unusableClaim says and is never matched as a
// prefix. A prefix counts the ids prefixCandidates gives, as tree.Resolve counts them: a prefix
// one unusable id shares with another id stays ambiguous, and a prefix of one unusable id alone
// fails as prefixMatch says. It reads through q.
func (x *Index) resolveID(q querier, nodes nodeTable, arg string) (string, error) {
	ctx := context.Background()
	name := tree.NormalizeArg(arg)
	if refused, err := x.refusedClaim(ctx, q, nodes, name); err != nil || refused != nil {
		if err != nil {
			return "", err
		}
		return "", refused
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
	if err := x.unusableClaim(ctx, q, nodes, arg, name); err != nil {
		return "", err
	}

	candidates, err := x.prefixCandidates(ctx, q, nodes, name)
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
// tree.Resolve counts them: the id of every row, left out for a repeated id or not, of every file
// with problems the refresh read, and of every node file the latest refresh left out below a
// repeated id. A file the walk leaves out unread or under an orphan directory is no node of the
// tree and counts as none. An empty name has none. It reads through q.
func (x *Index) prefixCandidates(ctx context.Context, q querier, nodes nodeTable, name string) ([]string, error) {
	if name == "" {
		return nil, nil
	}
	candidates, err := x.strings(ctx, q, `SELECT id FROM `+nodes.table+`
		WHERE substr(id, 1, length(?1)) = ?1`, name)
	if err != nil {
		return nil, err
	}
	// Every file with problems claims its id, so a prefix of that id is a prefix of one of its
	// claims; the claims rows of a file without a files row come from the walk.
	claimants, err := x.strings(ctx, q, `SELECT DISTINCT path FROM claims
		WHERE kind = ?1 AND substr(name, 1, length(?2)) = ?2 AND path IN (SELECT path FROM files)`,
		nodes.kind, name)
	if err != nil {
		return nil, err
	}
	for _, path := range claimants {
		if id := tree.IDOf(path); strings.HasPrefix(id, name) {
			candidates = append(candidates, id)
		}
	}
	candidates = append(candidates, x.leftOutPrefixCandidates(nodes.kind, name)...)
	slices.Sort(candidates)
	return slices.Compact(candidates), nil
}

// prefixMatch returns id, the one candidate the prefix arg matched, when a usable node of the kind
// has it. Otherwise it fails as the exact name id would, quoting arg where the refusal quotes the
// argument: with the duplicate problem of a node file left out for or below a repeated id, or
// with unusableClaim's refusal of a name a file with problems claims. It reads through q.
func (x *Index) prefixMatch(ctx context.Context, q querier, nodes nodeTable, arg, id string) (string, error) {
	if problem := x.leftOutProblem(nodes.kind, func(node leftOutNode) bool {
		return node.id == id
	}); problem != nil {
		return "", problem
	}
	usable, err := x.strings(ctx, q, `SELECT id FROM `+nodes.table+` WHERE id = ? AND excluded = 0`, id)
	if err != nil || len(usable) == 1 {
		return firstOf(usable), err
	}
	if refused, err := x.refusedClaim(ctx, q, nodes, id); err != nil || refused != nil {
		if err != nil {
			return "", err
		}
		return "", refused
	}
	if err := x.unusableClaim(ctx, q, nodes, arg, id); err != nil {
		return "", err
	}
	return "", fmt.Errorf("id %q %w", arg, tree.ErrNotFound)
}

// refusedClaim returns the problem that refuses name for every node of the kind, or nil when none
// does: a *tree.Error wrapping tree.ErrDuplicate. A node left out as a duplicate that has name as
// its id or an alias fails as duplicateClaim says; an alias two usable nodes claim, as an alias or
// one's id, fails with the problem the refresh warned about, naming both files; a name a row has
// that an unusable file also claims fails as unusableConflict says. It reads through q.
func (x *Index) refusedClaim(ctx context.Context, q querier, nodes nodeTable, name string) (*tree.Error, error) {
	duplicate, err := x.duplicateClaim(ctx, q, nodes, name)
	if err != nil || duplicate != nil {
		return duplicate, err
	}
	claims, err := readNameClaims(ctx, q, nameClaimsQuery(nodes)+` WHERE name = ?2`, nodes.kind, name)
	if err != nil {
		return nil, fmt.Errorf("%s: query: %w", Path(x.store), err)
	}
	if clashes := aliasClashes(claims); len(clashes) > 0 {
		return clashes[0], nil
	}
	return x.unusableConflict(ctx, q, nodes, name)
}

// unusableConflict returns the duplicate that refuses name when a file the claims table records
// claims it and a row of the kind has it as its id or an alias, or nil when that is not so:
// tree.Resolve refuses such a name before any lookup (the Resolution order). The problem is on the
// first such file by path, and names in Other the first row's file by path, both relative to the
// working tree root. When a claiming file is a node file the latest refresh left out below a
// repeated id, the problem is that id's duplicate problem instead. It reads through q.
func (x *Index) unusableConflict(ctx context.Context, q querier, nodes nodeTable, name string) (*tree.Error, error) {
	claimants, err := x.strings(ctx, q, `SELECT DISTINCT path FROM claims
		WHERE kind = ? AND name = ? ORDER BY path`, nodes.kind, name)
	if err != nil || len(claimants) == 0 {
		return nil, err
	}
	others, err := x.strings(ctx, q, `SELECT path FROM `+nodes.table+` WHERE id = ?2
		UNION SELECT path FROM aliases WHERE kind = ?1 AND alias = ?2
		ORDER BY path`, nodes.kind, name)
	if err != nil || len(others) == 0 {
		return nil, err
	}

	for _, claimant := range claimants {
		if problem := x.leftOutProblem(nodes.kind, func(node leftOutNode) bool {
			return node.path == claimant
		}); problem != nil {
			return problem, nil
		}
	}
	if tree.IDOf(claimants[0]) == name {
		return tree.DuplicateIDError(claimants[0], others[0], name), nil
	}
	return tree.DuplicateAliasError(claimants[0], others[0], name, tree.IDOf(others[0]) != name), nil
}

// unusableClaim returns why name, which no usable node of the kind answers to, still must not be
// matched as a prefix, or nil when nothing else claims it. A name a file of the kind with problems,
// a file the walk leaves out unread, or a node file under a directory the walk leaves out claims
// (its id or an alias, as the claims table records them), or an alias only the later file of a
// repeated id lists, fails with an error wrapping tree.ErrNotFound that quotes arg and names the
// first such file in path order, unless that file is a node file the latest refresh left out below
// a repeated id: then it fails with that id's duplicate problem, naming both of its files. It
// reads through q.
func (x *Index) unusableClaim(ctx context.Context, q querier, nodes nodeTable, arg, name string) error {
	// refusedClaim refuses every other alias of a row left out for a repeated id, so an alias such a
	// row lists is here only the later file's.
	invalid, err := x.strings(ctx, q, `SELECT path FROM claims WHERE kind = ?1 AND name = ?2
		UNION SELECT aliases.path FROM aliases
			JOIN `+nodes.table+` AS node ON node.path = aliases.path
			WHERE aliases.kind = ?1 AND aliases.alias = ?2 AND node.excluded = 1
		ORDER BY path LIMIT 1`, nodes.kind, name)
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
// out below a repeated id that name is a prefix of.
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
// an alias, or nil when no left-out node has: only a repeated id leaves nodes out. An alias only a
// later file of a repeated id lists, which no other file claims, is no duplicate: unusableClaim
// refuses it, as tree.Resolve does. The problem is the one on name itself when two files claim it,
// otherwise one on another name the node shares with a different file; both files are relative to
// the working tree root. It reads through q.
func (x *Index) duplicateClaim(ctx context.Context, q querier, nodes nodeTable, name string) (*tree.Error, error) {
	rows, err := q.QueryContext(ctx, `WITH claims(name, path, kind) AS (
			SELECT id, path, 'id' FROM `+nodes.table+`
			UNION ALL SELECT alias, path, 'alias' FROM aliases WHERE kind = ?2
		)
		SELECT mine.path, mine.kind, mine.name, theirs.path, theirs.kind
		FROM claims AS mine
		JOIN claims AS theirs ON theirs.name = mine.name AND theirs.path <> mine.path
		WHERE mine.path IN (
			SELECT claims.path FROM claims JOIN `+nodes.table+` AS node ON node.path = claims.path
			WHERE claims.name = ?1 AND node.excluded = 1 AND (
				claims.kind = 'id'
				OR NOT EXISTS (SELECT 1 FROM `+nodes.table+` AS earlier
					WHERE earlier.id = node.id AND earlier.excluded = 1 AND earlier.path < node.path)
				OR EXISTS (SELECT 1 FROM claims AS other
					WHERE other.name = ?1 AND other.path <> claims.path)
			)
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
