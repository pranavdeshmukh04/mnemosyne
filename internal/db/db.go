package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	// Ensure the parent directory exists so SQLite can create the file.
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create directory %s: %w", dir, err)
	}

	// Open (or create) the SQLite database file.
	// "sqlite" is the driver name registered by modernc.org/sqlite.
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Verify the connection is actually usable.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	// Enable WAL (Write-Ahead Logging) mode.
	// Default SQLite uses "delete" journal mode which locks the whole DB
	// during writes. WAL writes changes to a separate log file instead,
	// allowing concurrent readers (the viewer) and writers (the MCP server)
	// without blocking each other.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	// Enable foreign key enforcement. SQLite has this OFF by default and
	// it's a per-connection setting (not persisted in the DB file), so we
	// must set it every time we open a connection. Without this,
	// REFERENCES and ON DELETE CASCADE are silently ignored.
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	return db, nil
}
