package db

import (
	"database/sql"
	"fmt"
)

// Migrate runs the schema DDL. Every statement uses IF NOT EXISTS,
// so calling Migrate multiple times on the same DB is safe (idempotent).
func Migrate(db *sql.DB) error {
	// We wrap all statements in a single transaction so the schema is
	// applied atomically — either everything succeeds or nothing changes.
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration tx: %w", err)
	}
	defer tx.Rollback() // no-op if tx.Commit() is reached first

	for _, stmt := range schema {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("exec migration: %w\nstatement: %s", err, stmt)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration tx: %w", err)
	}
	return nil
}

// schema holds every DDL statement in order. Each uses IF NOT EXISTS
// so re-running is harmless.
var schema = []string{
	// --- nodes table ---
	// Stores both entities (files, decisions, bugs...) and conversations.
	// 'kind' distinguishes them. 'metadata' is a free-form JSON blob for
	// kind-specific fields (e.g., line range for a function node).
	`CREATE TABLE IF NOT EXISTS nodes (
		id         TEXT PRIMARY KEY,
		kind       TEXT NOT NULL,
		name       TEXT NOT NULL,
		body       TEXT,
		metadata   TEXT,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_nodes_kind ON nodes(kind)`,
	`CREATE INDEX IF NOT EXISTS idx_nodes_name ON nodes(name)`,

	// --- edges table ---
	// Directed, typed relationships between nodes.
	// ON DELETE CASCADE means deleting a node automatically removes
	// all edges pointing to/from it — no orphaned edges.
	`CREATE TABLE IF NOT EXISTS edges (
		id         TEXT PRIMARY KEY,
		src        TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
		dst        TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
		kind       TEXT NOT NULL,
		metadata   TEXT,
		created_at INTEGER NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_edges_src  ON edges(src)`,
	`CREATE INDEX IF NOT EXISTS idx_edges_dst  ON edges(dst)`,
	`CREATE INDEX IF NOT EXISTS idx_edges_kind ON edges(kind)`,

	// --- transcripts table ---
	// Stores the full text of a conversation session.
	// Foreign key links it 1:1 to a conversation node.
	`CREATE TABLE IF NOT EXISTS transcripts (
		conversation_id TEXT PRIMARY KEY REFERENCES nodes(id) ON DELETE CASCADE,
		content         TEXT NOT NULL,
		created_at      INTEGER NOT NULL
	)`,

	// --- project table ---
	// Simple key-value store for project-level settings like name,
	// description, and schema version. Typically just a few rows.
	`CREATE TABLE IF NOT EXISTS project (
		key   TEXT PRIMARY KEY,
		value TEXT
	)`,

	// Seed the schema version so future migrations can check it.
	`INSERT OR IGNORE INTO project (key, value) VALUES ('schema_version', '1')`,

	// Enable foreign key enforcement. SQLite has this OFF by default
	// for backwards compatibility. Without this, ON DELETE CASCADE
	// and REFERENCES constraints are silently ignored.
	`PRAGMA foreign_keys = ON`,
}
