package index

import (
	"errors"
	"fmt"

	"github.com/airiclenz/capcom/internal/diagnose"
)

// claim is one name a node answers to: its id or one of its Aliases.
type claim struct {
	name string
	path string
	kind string
}

// excludeDuplicates keeps every node that shares an id or alias with another file of its own tree
// out of the queries and warns about each, naming the other file (ADR 0002: names are unique
// across the tree). An issue and a Note never clash. It runs on every refresh, so a node comes
// back once its rival is gone.
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
	rows, err := r.tx.QueryContext(r.ctx, `SELECT id, path, 'id' FROM `+nodes.table+`
		UNION ALL SELECT alias, path, 'alias' FROM aliases WHERE kind = ? ORDER BY 1, 2`, nodes.kind)
	if err != nil {
		return fmt.Errorf("read indexed names: %w", err)
	}
	var claims []diagnose.Claim
	for rows.Next() {
		var current diagnose.Claim
		if err := rows.Scan(&current.Name, &current.Path, &current.Kind); err != nil {
			rows.Close()
			return fmt.Errorf("read indexed names: %w", err)
		}
		claims = append(claims, current)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return fmt.Errorf("read indexed names: %w", err)
	}

	if _, err := r.tx.ExecContext(r.ctx, `UPDATE `+nodes.table+` SET excluded = 0`); err != nil {
		return fmt.Errorf("reset excluded %s: %w", nodes.table, err)
	}
	return r.excludeClaims(nodes, diagnose.NameClashes(claims))
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
