package index

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/airiclenz/capcom/internal/diagnose"
	"github.com/airiclenz/capcom/internal/tree"
)

// The kinds of a name claim: a node answers to its id and to each of its Aliases.
const (
	claimID    = "id"
	claimAlias = "alias"
)

// claim is one name a node answers to: its id or one of its Aliases.
type claim struct {
	name string
	path string
	kind string
}

// nameClaim is one name a row of one kind answers to, as diagnose.NameClashes takes it, with the
// row's id and whether the row is left out for a repeated id: aliasClashes orders by both.
type nameClaim struct {
	diagnose.Claim
	id         string
	isExcluded bool
}

// excludeDuplicates keeps every node of an id another file of its own tree also has out of the
// queries and warns about each once, naming another file of the id (ADR 0002: names are unique
// across the tree). An alias two files claim, as an alias or one's id, leaves both nodes in: only
// the alias is refused, and the refresh warns once per clash, at the file tree.Load reports it on.
// An issue and a Note never clash. It runs on every refresh, so a node comes back once its rival is
// gone.
func (r *refresher) excludeDuplicates() error {
	for _, nodes := range []nodeTable{issueNodes, noteNodes} {
		if err := r.excludeDuplicatesOf(nodes); err != nil {
			return err
		}
	}
	return nil
}

// excludeDuplicatesOf runs excludeDuplicates over the nodes of one kind.
func (r *refresher) excludeDuplicatesOf(nodes nodeTable) error {
	claims, err := readNameClaims(r.ctx, r.tx, nameClaimsQuery(nodes), nodes.kind)
	if err != nil {
		return fmt.Errorf("read indexed names: %w", err)
	}
	if _, err := r.tx.ExecContext(r.ctx, `UPDATE `+nodes.table+` SET excluded = 0`); err != nil {
		return fmt.Errorf("reset excluded %s: %w", nodes.table, err)
	}

	repeated := repeatedIDClaims(claims)
	excluded := map[string]bool{}
	for _, current := range repeated {
		excluded[current.Path] = true
	}
	for i := range claims {
		claims[i].isExcluded = excluded[claims[i].Path]
	}
	for _, problem := range aliasClashes(claims) {
		r.warnings = append(r.warnings, Warning{Kind: diagnose.Duplicate, Path: problem.Path, Message: problem.Message()})
	}
	return r.excludeClaims(nodes, diagnose.NameClashes(repeated))
}

// excludeClaims warns about every name clash and excludes each file it names, once.
func (r *refresher) excludeClaims(nodes nodeTable, clashes []Warning) error {
	r.warnings = append(r.warnings, clashes...)
	excluded := map[string]bool{}
	for _, clash := range clashes {
		if excluded[clash.Path] {
			continue
		}
		excluded[clash.Path] = true
		if _, err := r.tx.ExecContext(r.ctx, `UPDATE `+nodes.table+` SET excluded = 1 WHERE path = ?`, clash.Path); err != nil {
			return fmt.Errorf("exclude %s: %w", clash.Path, err)
		}
	}
	return nil
}

// nameClaimsQuery selects every name the rows of one kind answer to, ordered by name and then
// path, as name, path, kind, the row's id and its excluded flag; ?1 is the kind. A caller may
// append a WHERE clause on those columns.
func nameClaimsQuery(nodes nodeTable) string {
	return `WITH claims(name, path, kind, id, excluded) AS (
			SELECT id, path, '` + claimID + `', id, excluded FROM ` + nodes.table + `
			UNION ALL SELECT aliases.alias, aliases.path, '` + claimAlias + `', node.id, node.excluded
				FROM aliases JOIN ` + nodes.table + ` AS node ON node.path = aliases.path
				WHERE aliases.kind = ?1
		)
		SELECT name, path, kind, id, excluded FROM claims`
}

// readNameClaims reads the claims query selects through q, in its order.
func readNameClaims(ctx context.Context, q querier, query string, args ...any) ([]nameClaim, error) {
	rows, err := q.QueryContext(ctx, query+` ORDER BY name, path, kind`, args...)
	if err != nil {
		return nil, err
	}
	var claims []nameClaim
	for rows.Next() {
		var current nameClaim
		if err := rows.Scan(&current.Name, &current.Path, &current.Kind, &current.id, &current.isExcluded); err != nil {
			rows.Close()
			return nil, err
		}
		claims = append(claims, current)
	}
	return claims, errors.Join(rows.Err(), rows.Close())
}

// repeatedIDClaims returns the id claims of every id two or more of claims share, in their order.
func repeatedIDClaims(claims []nameClaim) []diagnose.Claim {
	idCounts := map[string]int{}
	for _, current := range claims {
		if current.Kind == claimID {
			idCounts[current.Name]++
		}
	}
	var repeated []diagnose.Claim
	for _, current := range claims {
		if current.Kind == claimID && idCounts[current.Name] > 1 {
			repeated = append(repeated, current.Claim)
		}
	}
	return repeated
}

// aliasClashes returns the problem of every alias two or more of claims share that no two of them
// have as their id, the way tree.Load reports it: the node that has the alias as its id, else the
// first that lists it, owns it, and every other file listing it has a problem naming the owner.
// Files are taken as tree.Load takes them: the rows left out for a repeated id after the others,
// each by id and then path. Problems follow the order of claims by name.
func aliasClashes(claims []nameClaim) []*tree.Error {
	var names []string
	byName := map[string][]nameClaim{}
	for _, current := range claims {
		if _, isNamed := byName[current.Name]; !isNamed {
			names = append(names, current.Name)
		}
		byName[current.Name] = append(byName[current.Name], current)
	}

	var problems []*tree.Error
	for _, name := range names {
		problems = append(problems, aliasClashesOf(name, byName[name])...)
	}
	return problems
}

// aliasClashesOf returns the problems of name, which claimants share, as aliasClashes words them.
// A name two or more claimants have as their id yields none: those rows are left out instead.
func aliasClashesOf(name string, claimants []nameClaim) []*tree.Error {
	var owner *nameClaim
	for i, current := range claimants {
		if current.Kind != claimID {
			continue
		}
		if owner != nil {
			return nil
		}
		owner = &claimants[i]
	}

	listers := slices.DeleteFunc(slices.Clone(claimants), func(current nameClaim) bool {
		return current.Kind == claimID
	})
	slices.SortFunc(listers, func(a, b nameClaim) int {
		if a.isExcluded != b.isExcluded {
			if a.isExcluded {
				return 1
			}
			return -1
		}
		return cmp.Or(cmp.Compare(a.id, b.id), cmp.Compare(a.Path, b.Path))
	})
	var problems []*tree.Error
	for i, current := range listers {
		if owner == nil {
			owner = &listers[i]
			continue
		}
		problems = append(problems, tree.DuplicateAliasError(current.Path, owner.Path, name, owner.Kind == claimAlias))
	}
	return problems
}
