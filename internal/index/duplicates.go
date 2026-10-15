package index

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path"

	"github.com/airiclenz/capcom/internal/diagnose"
	"github.com/airiclenz/capcom/internal/tree"
)

// claim is one name a node file claims: its id or one of its Aliases, with how far the file was
// read for its aliases.
type claim struct {
	name  string
	path  string
	kind  string
	parse tree.ParseState
}

// The verdicts of a walk file, kept in file_verdicts.verdict: a node of the tree, or left out for a
// repeated id it has or lies below.
const (
	verdictKept            = "kept"
	verdictRepeatedID      = "repeated-id"
	verdictBelowRepeatedID = "below-repeated-id"
)

// The refusals of a name, kept in name_verdicts.refusal: two files claim it, or only an unusable
// file does.
const (
	refusalDuplicate = "duplicate"
	refusalUnusable  = "unusable"
)

// parseStateNames are the claims.parse of each tree.ParseState a file that claims names has.
var parseStateNames = map[tree.ParseState]string{
	tree.ParsedFile:   "parsed",
	tree.UnparsedFile: "unparsed",
	tree.UnreadFile:   "unread",
}

// parseStateNamed returns the tree.ParseState stored as name in claims.parse, or zero for a name
// parseStateNames has not.
func parseStateNamed(name string) tree.ParseState {
	for state, stateName := range parseStateNames {
		if stateName == name {
			return state
		}
	}
	return 0
}

// leftOutVerdicts are the file_verdicts.verdict of each reason tree.DecideMembership leaves a walk
// file out for.
var leftOutVerdicts = map[tree.LeftOutReason]string{
	tree.RepeatedIDFile:  verdictRepeatedID,
	tree.BelowRepeatedID: verdictBelowRepeatedID,
}

// refusalNames are the name_verdicts.refusal of each reason tree.DecideMembership refuses a name
// for.
var refusalNames = map[tree.RefusalReason]string{
	tree.DuplicateRefusal: refusalDuplicate,
	tree.UnusableRefusal:  refusalUnusable,
}

// kindFacts are the facts of the files of one kind that tree.DecideMembership decides from, and
// which of them are clean: parsed, valid files with their own node rows.
type kindFacts struct {
	facts   []tree.FileFact
	isClean map[string]bool
}

// persistMembership replaces the stored membership verdicts with tree.DecideMembership's verdicts
// on the files of each kind, as the rows of this refresh record them, and warns about every
// repeated id and every alias two clean files claim (ADR 0002: names are unique across the tree).
// It runs after replaceWalkClaims, once the files and claims rows are current, so a node comes back
// once its rival is gone and a clean file is never read again for it. An issue and a Note never
// clash.
func (r *refresher) persistMembership() error {
	for _, table := range []string{"file_verdicts", "name_verdicts"} {
		if _, err := r.tx.ExecContext(r.ctx, `DELETE FROM `+table); err != nil {
			return fmt.Errorf("clear %s: %w", table, err)
		}
	}
	for _, nodes := range []nodeTable{issueNodes, noteNodes} {
		if err := r.persistMembershipOf(nodes); err != nil {
			return err
		}
	}
	return nil
}

// persistMembershipOf runs persistMembership over the files of one kind and marks each node row
// left out for a repeated id as excluded.
func (r *refresher) persistMembershipOf(nodes nodeTable) error {
	read, err := r.membershipFacts(nodes.kind)
	if err != nil {
		return fmt.Errorf("read indexed names: %w", err)
	}
	membership := tree.DecideMembership(read.facts)
	r.warnings = append(r.warnings, diagnose.NameClashes(membership.IDConflicts)...)
	for _, conflict := range membership.AliasConflicts {
		// A file with problems is warned about by its own problems, and a file left out below a
		// repeated id by the directory it lies in: the warnings stay a function of the clean files.
		if !read.isClean[conflict.Key] {
			continue
		}
		other, isOtherAlias := conflict.Other, conflict.IsOtherAlias
		if !read.isClean[other] {
			claimant, isFound := read.firstCleanClaimant(membership.Names[conflict.Name].Claims, conflict.Key)
			if !isFound {
				continue
			}
			other, isOtherAlias = read.facts[claimant.Fact].Key, claimant.IsAlias
		}
		problem := tree.DuplicateAliasError(conflict.Key, other, conflict.Name, isOtherAlias)
		r.warn(diagnose.Duplicate, problem.Path, 0, problem.Message())
	}

	if err := r.insertFileVerdicts(nodes.kind, read.facts, membership); err != nil {
		return err
	}
	if err := r.insertNameVerdicts(nodes.kind, membership); err != nil {
		return err
	}
	_, err = r.tx.ExecContext(r.ctx, `UPDATE `+nodes.table+` SET excluded = EXISTS (
		SELECT 1 FROM file_verdicts AS stored
		WHERE stored.path = `+nodes.table+`.path AND stored.verdict <> ?)`, verdictKept)
	if err != nil {
		return fmt.Errorf("exclude left-out %s: %w", nodes.table, err)
	}
	return nil
}

// firstCleanClaimant returns the first of claims, a name's claims in DecideMembership's order,
// whose file is clean and comes before the file key, and whether there is one.
func (k kindFacts) firstCleanClaimant(claims []tree.NameClaim, key string) (tree.NameClaim, bool) {
	for _, claim := range claims {
		claimKey := k.facts[claim.Fact].Key
		if claimKey == key {
			break
		}
		if k.isClean[claimKey] {
			return claim, true
		}
	}
	return tree.NameClaim{}, false
}

// membershipFacts builds the facts of every file of kind from the rows of this refresh, reading no
// file. Every files row is a walk file: a clean one with the aliases its aliases rows record, a
// file with problems with the aliases and the parse state its claims rows record, which is
// unusable. Every node file the walk left out below a repeated id is a walk file too, with the
// aliases the claims of its orphan directory record. Every other claims row is a claim-only
// file's, with its parse state: a file the walk refused to read or a node file under an orphan
// directory, which refuses names and never leaves a file out.
func (r *refresher) membershipFacts(kind string) (kindFacts, error) {
	read := kindFacts{isClean: map[string]bool{}}
	aliasesOf, _, err := namesByPath(r.ctx, r.tx,
		`SELECT path, alias FROM aliases WHERE kind = ? ORDER BY rowid`, kind)
	if err != nil {
		return kindFacts{}, err
	}
	claimsOf, claimPaths, err := namesByPath(r.ctx, r.tx,
		`SELECT path, name FROM claims WHERE kind = ? ORDER BY rowid`, kind)
	if err != nil {
		return kindFacts{}, err
	}
	// Every claims row of a file records the same parse state.
	parseNamesOf, _, err := namesByPath(r.ctx, r.tx,
		`SELECT DISTINCT path, parse FROM claims WHERE kind = ?`, kind)
	if err != nil {
		return kindFacts{}, err
	}
	parseOf := make(map[string]tree.ParseState, len(parseNamesOf))
	for filePath, names := range parseNamesOf {
		parseOf[filePath] = parseStateNamed(names[0])
	}
	files, err := r.indexedFiles(kind)
	if err != nil {
		return kindFacts{}, err
	}

	isTaken := map[string]bool{}
	for _, file := range files {
		isTaken[file.path] = true
		id := tree.IDOf(file.path)
		if !file.hasProblems {
			read.isClean[file.path] = true
			read.facts = append(read.facts, walkFact(file.path, id, aliasesOf[file.path], tree.ParsedFile))
			continue
		}
		fact := walkFact(file.path, id, claimedAliases(file.path, claimsOf[file.path]), parseOf[file.path])
		fact.IsUsable = false
		read.facts = append(read.facts, fact)
	}
	for _, node := range r.leftOut {
		if node.kind != kind {
			continue
		}
		isTaken[node.path] = true
		aliases := claimedAliases(node.path, claimsOf[node.path])
		read.facts = append(read.facts, walkFact(node.path, node.id, aliases, tree.ParsedFile))
	}
	for _, claimPath := range claimPaths {
		if isTaken[claimPath] {
			continue
		}
		aliases := claimedAliases(claimPath, claimsOf[claimPath])
		fact := walkFact(claimPath, tree.IDOf(claimPath), aliases, parseOf[claimPath])
		fact.Origin = tree.ClaimOnlyOrigin
		fact.IsUsable = false
		read.facts = append(read.facts, fact)
	}
	return read, nil
}

// walkFact returns the fact of the usable walk file at key, a slash path relative to the working
// tree root, with its id, aliases and parse state.
func walkFact(key, id string, aliases []string, parse tree.ParseState) tree.FileFact {
	return tree.FileFact{
		Key:      key,
		ID:       id,
		Dir:      path.Dir(key),
		Aliases:  aliases,
		Parse:    parse,
		IsUsable: true,
		Origin:   tree.WalkOrigin,
	}
}

// claimedAliases returns the aliases among names, the claims of the file at filePath, that
// tree.CheckAliases passes: its id and any name it repeats are none.
func claimedAliases(filePath string, names []string) []string {
	aliases, _ := tree.CheckAliases(filePath, "node", tree.IDOf(filePath), names)
	return aliases
}

// indexedFile is a files row of one kind: the file's path, and whether it has problems.
type indexedFile struct {
	path        string
	hasProblems bool
}

// indexedFiles reads every files row of kind, in path order.
func (r *refresher) indexedFiles(kind string) ([]indexedFile, error) {
	rows, err := r.tx.QueryContext(r.ctx,
		`SELECT path, problems IS NOT NULL FROM files WHERE kind = ? ORDER BY path`, kind)
	if err != nil {
		return nil, err
	}
	var files []indexedFile
	for rows.Next() {
		var file indexedFile
		if err := rows.Scan(&file.path, &file.hasProblems); err != nil {
			rows.Close()
			return nil, err
		}
		files = append(files, file)
	}
	return files, errors.Join(rows.Err(), rows.Close())
}

// namesByPath reads the path and name pairs query selects through tx: the names of each path in
// their order, and the paths in the order each first appears.
func namesByPath(ctx context.Context, tx *sql.Tx, query string, args ...any) (map[string][]string, []string, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	names := map[string][]string{}
	var paths []string
	for rows.Next() {
		var filePath, name string
		if err := rows.Scan(&filePath, &name); err != nil {
			rows.Close()
			return nil, nil, err
		}
		if _, isSeen := names[filePath]; !isSeen {
			paths = append(paths, filePath)
		}
		names[filePath] = append(names[filePath], name)
	}
	return names, paths, errors.Join(rows.Err(), rows.Close())
}

// insertFileVerdicts stores the verdict of every walk file of kind among facts: kept, or left out
// and why.
func (r *refresher) insertFileVerdicts(kind string, facts []tree.FileFact, membership tree.Membership) error {
	insert, err := r.tx.PrepareContext(r.ctx, `INSERT INTO file_verdicts (path, kind, id, verdict) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("store file verdicts: %w", err)
	}
	defer insert.Close()
	record := func(i int, verdict string) error {
		if _, err := insert.ExecContext(r.ctx, facts[i].Key, kind, facts[i].ID, verdict); err != nil {
			return fmt.Errorf("store the verdict of %s: %w", facts[i].Key, err)
		}
		return nil
	}
	for _, i := range membership.Kept {
		if err := record(i, verdictKept); err != nil {
			return err
		}
	}
	for _, file := range membership.LeftOut {
		if err := record(file.Fact, leftOutVerdicts[file.Reason]); err != nil {
			return err
		}
	}
	return nil
}

// insertNameVerdicts stores every name of kind membership refuses, with why and the conflict that
// refuses it.
func (r *refresher) insertNameVerdicts(kind string, membership tree.Membership) error {
	insert, err := r.tx.PrepareContext(r.ctx, `INSERT INTO name_verdicts
		(kind, name, refusal, path, other, is_alias, is_other_alias) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("store name verdicts: %w", err)
	}
	defer insert.Close()
	for name, verdict := range membership.Names {
		if verdict.Refusal == 0 {
			continue
		}
		conflict := verdict.Conflict
		_, err := insert.ExecContext(
			r.ctx, kind, name, refusalNames[verdict.Refusal],
			conflict.Key, conflict.Other, conflict.IsAlias, conflict.IsOtherAlias,
		)
		if err != nil {
			return fmt.Errorf("store the verdict of %q: %w", name, err)
		}
	}
	return nil
}
