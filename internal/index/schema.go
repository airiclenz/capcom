package index

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"

	"github.com/airiclenz/capcom/internal/config"
)

// schemaVersion identifies the table layout below. Bump it whenever the layout or the meaning of a
// column changes: an index recorded under another version is dropped and rebuilt.
const schemaVersion = 1

// Keys of the meta table.
const (
	metaSchemaVersion = "schema_version"
	metaConfigHash    = "config_hash"
)

// kindIssue is the files.kind of an issue file.
const kindIssue = "issue"

// schemaStatements create the index tables. Every row hangs off a files row by path, the
// worktree-relative path of the file it came from; issues.excluded marks a parsed issue that a
// duplicate id or alias keeps out of every query.
var schemaStatements = []string{
	`CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
	`CREATE TABLE files (
		path TEXT PRIMARY KEY,
		kind TEXT NOT NULL,
		size INTEGER NOT NULL,
		mtime INTEGER NOT NULL,
		refreshed INTEGER NOT NULL,
		problems TEXT
	)`,
	`CREATE TABLE issues (
		path TEXT PRIMARY KEY,
		id TEXT NOT NULL,
		parent TEXT NOT NULL,
		status TEXT NOT NULL,
		category TEXT NOT NULL,
		type TEXT NOT NULL,
		priority INTEGER NOT NULL,
		plan TEXT NOT NULL,
		created TEXT NOT NULL,
		closed TEXT NOT NULL,
		reason TEXT NOT NULL,
		title TEXT NOT NULL,
		excluded INTEGER NOT NULL DEFAULT 0
	)`,
	`CREATE INDEX issues_by_id ON issues (id)`,
	`CREATE INDEX issues_by_parent ON issues (parent)`,
	`CREATE TABLE labels (path TEXT NOT NULL, label TEXT NOT NULL)`,
	`CREATE INDEX labels_by_path ON labels (path)`,
	`CREATE TABLE deps (path TEXT NOT NULL, from_id TEXT NOT NULL, to_id TEXT NOT NULL)`,
	`CREATE INDEX deps_by_path ON deps (path)`,
	`CREATE TABLE aliases (path TEXT NOT NULL, alias TEXT NOT NULL)`,
	`CREATE INDEX aliases_by_path ON aliases (path)`,
	`CREATE VIRTUAL TABLE issues_fts USING fts5 (path UNINDEXED, title, body)`,
}

// rowTables are the tables holding one file's rows, cleared by path when the file changes or goes.
var rowTables = []string{"files", "issues", "labels", "deps", "aliases", "issues_fts"}

// configHash fingerprints the parts of the config the indexed rows depend on: the status map
// (each status's category is stored, ADR 0003) and the declared types (Validate decides which
// files are excluded by them).
func configHash(cfg config.Config) string {
	hash := sha256.New()
	for _, status := range cfg.Statuses {
		fmt.Fprintf(hash, "status %q %q\n", status.Name, status.Category)
	}
	for _, issueType := range cfg.Types {
		fmt.Fprintf(hash, "type %q\n", issueType)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// ensureSchema makes the database match the current schema version and config hash, dropping and
// recreating every table when it does not or when isForced is set. It reports whether it rebuilt.
func ensureSchema(ctx context.Context, tx *sql.Tx, hash string, isForced bool) (bool, error) {
	if !isForced {
		isCurrent, err := isSchemaCurrent(ctx, tx, hash)
		if err != nil || isCurrent {
			return false, err
		}
	}
	if err := dropTables(ctx, tx); err != nil {
		return false, err
	}
	for _, statement := range schemaStatements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return false, fmt.Errorf("create index schema: %w", err)
		}
	}
	meta := map[string]string{
		metaSchemaVersion: strconv.Itoa(schemaVersion),
		metaConfigHash:    hash,
	}
	for key, value := range meta {
		if _, err := tx.ExecContext(ctx, `INSERT INTO meta (key, value) VALUES (?, ?)`, key, value); err != nil {
			return false, fmt.Errorf("write index meta: %w", err)
		}
	}
	return true, nil
}

// isSchemaCurrent reports whether the database was recorded under this schema version and config
// hash. A database without a meta table, a new one included, is not current.
func isSchemaCurrent(ctx context.Context, tx *sql.Tx, hash string) (bool, error) {
	var tables int
	err := tx.QueryRowContext(
		ctx,
		`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'meta'`,
	).Scan(&tables)
	if err != nil || tables == 0 {
		return false, err
	}

	recorded := map[string]string{}
	rows, err := tx.QueryContext(ctx, `SELECT key, value FROM meta`)
	if err != nil {
		return false, fmt.Errorf("read index meta: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return false, fmt.Errorf("read index meta: %w", err)
		}
		recorded[key] = value
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("read index meta: %w", err)
	}
	return recorded[metaSchemaVersion] == strconv.Itoa(schemaVersion) && recorded[metaConfigHash] == hash, nil
}

// dropTables drops every table, whatever schema version created it. Virtual tables go first,
// since dropping one also drops its shadow tables.
func dropTables(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `SELECT name FROM sqlite_master
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%'
		ORDER BY sql LIKE 'CREATE VIRTUAL TABLE%' DESC, name`)
	if err != nil {
		return fmt.Errorf("list index tables: %w", err)
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return fmt.Errorf("list index tables: %w", err)
		}
		names = append(names, name)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return fmt.Errorf("list index tables: %w", err)
	}
	for _, name := range names {
		if _, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS "`+name+`"`); err != nil {
			return fmt.Errorf("drop index table %s: %w", name, err)
		}
	}
	return nil
}
