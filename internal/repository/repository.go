// Package repository owns all SQLite access.
package repository

import (
	"database/sql"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Repository is a handle to the local SQLite store.
type Repository struct {
	db *sql.DB
}

// Open ensures the home directory and database file exist, opens a pooled
// SQLite connection in WAL mode with foreign keys enabled, and applies any
// pending migrations.
func Open(home string) (*Repository, error) {
	if err := os.MkdirAll(home, 0o755); err != nil {
		return nil, fmt.Errorf("create home dir: %w", err)
	}

	dbPath := filepath.Join(home, "db.sqlite")
	dsn := buildDSN(dbPath)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	migFS, err := fs.Sub(migrationsFS, ".")
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("locate migrations: %w", err)
	}
	if err := runMigrations(db, migFS); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Repository{db: db}, nil
}

// Close releases the underlying database connection pool.
func (r *Repository) Close() error {
	if r.db == nil {
		return nil
	}
	return r.db.Close()
}

// DB exposes the underlying *sql.DB. Phase 0 callers and tests use this; later
// phases will add typed methods and stop reaching into the pool directly.
func (r *Repository) DB() *sql.DB { return r.db }

func buildDSN(path string) string {
	q := url.Values{}
	q.Set("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "busy_timeout(5000)")
	return fmt.Sprintf("file:%s?%s", path, q.Encode())
}
