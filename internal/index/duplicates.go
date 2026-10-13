package index

import (
	"errors"
	"fmt"

	"github.com/airiclenz/capcom/internal/tree"
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
	byName := map[string][]claim{}
	var names []string
	for rows.Next() {
		var current claim
		if err := rows.Scan(&current.name, &current.path, &current.kind); err != nil {
			rows.Close()
			return fmt.Errorf("read indexed names: %w", err)
		}
		if _, isSeen := byName[current.name]; !isSeen {
			names = append(names, current.name)
		}
		byName[current.name] = append(byName[current.name], current)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return fmt.Errorf("read indexed names: %w", err)
	}

	if _, err := r.tx.ExecContext(r.ctx, `UPDATE `+nodes.table+` SET excluded = 0`); err != nil {
		return fmt.Errorf("reset excluded %s: %w", nodes.table, err)
	}
	for _, name := range names {
		if err := r.excludeClaims(nodes, byName[name]); err != nil {
			return err
		}
	}
	return nil
}

// excludeClaims excludes and warns about every claimant of one name when there is more than one.
func (r *refresher) excludeClaims(nodes nodeTable, claims []claim) error {
	if len(claims) < 2 {
		return nil
	}
	for _, current := range claims {
		for _, other := range claims {
			if other == current {
				continue
			}
			r.warn(WarningDuplicate, current.path, 0, fmt.Sprintf(
				"%v %s %q: also the %s of %s; both are left out",
				tree.ErrDuplicate, current.kind, current.name, other.kind, other.path,
			))
		}
		if _, err := r.tx.ExecContext(r.ctx, `UPDATE `+nodes.table+` SET excluded = 1 WHERE path = ?`, current.path); err != nil {
			return fmt.Errorf("exclude %s: %w", current.path, err)
		}
	}
	return nil
}
