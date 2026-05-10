package repository

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

const createSchemaVersionTable = `
CREATE TABLE IF NOT EXISTS schema_version (
	version    INTEGER PRIMARY KEY,
	applied_at INTEGER NOT NULL
)`

// runMigrations applies any pending NNNN_*.sql files in fsys against db.
// Each migration runs in its own transaction. The schema_version table is
// created on demand and tracks applied migrations.
func runMigrations(db *sql.DB, fsys fs.FS) error {
	if _, err := db.Exec(createSchemaVersionTable); err != nil {
		return fmt.Errorf("create schema_version: %w", err)
	}
	current, err := currentSchemaVersion(db)
	if err != nil {
		return err
	}
	files, err := listMigrationFiles(fsys)
	if err != nil {
		return err
	}
	for _, f := range files {
		if f.version <= current {
			continue
		}
		if err := applyMigration(db, fsys, f); err != nil {
			return fmt.Errorf("apply %s: %w", f.name, err)
		}
	}
	return nil
}

func currentSchemaVersion(db *sql.DB) (int, error) {
	var v sql.NullInt64
	if err := db.QueryRow("SELECT MAX(version) FROM schema_version").Scan(&v); err != nil {
		return 0, fmt.Errorf("query schema_version: %w", err)
	}
	if !v.Valid {
		return 0, nil
	}
	return int(v.Int64), nil
}

type migrationFile struct {
	version int
	name    string
}

func listMigrationFiles(fsys fs.FS) ([]migrationFile, error) {
	var out []migrationFile
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".sql") {
			return nil
		}
		base := pathBase(path)
		ver, err := parseLeadingVersion(base)
		if err != nil {
			// Skip files like .keep.sql that don't start with a number.
			return nil
		}
		out = append(out, migrationFile{version: ver, name: path})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}

func pathBase(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func parseLeadingVersion(name string) (int, error) {
	idx := strings.Index(name, "_")
	if idx <= 0 {
		return 0, fmt.Errorf("missing leading number")
	}
	n, err := strconv.Atoi(name[:idx])
	if err != nil {
		return 0, fmt.Errorf("parse leading number: %w", err)
	}
	return n, nil
}

func applyMigration(db *sql.DB, fsys fs.FS, f migrationFile) error {
	body, err := fs.ReadFile(fsys, f.name)
	if err != nil {
		return fmt.Errorf("read %s: %w", f.name, err)
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	if _, err := tx.Exec(string(body)); err != nil {
		_ = tx.Rollback()
		return err
	}
	_, err = tx.Exec(
		"INSERT INTO schema_version(version, applied_at) VALUES (?, ?)",
		f.version, time.Now().Unix(),
	)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
