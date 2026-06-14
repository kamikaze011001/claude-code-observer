package repository

import (
	"database/sql"
	"io/fs"
	"testing"
	"testing/fstest"

	_ "modernc.org/sqlite"
)

func openMemory(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestRunMigrations_EmptyFS_CreatesSchemaVersionTable(t *testing.T) {
	db := openMemory(t)
	if err := runMigrations(db, fstest.MapFS{}); err != nil {
		t.Fatalf("runMigrations: %v", err)
	}
	var v int
	err := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&v)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if v != 0 {
		t.Errorf("version = %d, want 0", v)
	}
}

func TestRunMigrations_AppliesValidMigration(t *testing.T) {
	db := openMemory(t)
	fsys := fstest.MapFS{
		"migrations/0001_test.sql": &fstest.MapFile{
			Data: []byte(`CREATE TABLE foo (id INTEGER PRIMARY KEY)`),
		},
	}
	if err := runMigrations(db, fsys); err != nil {
		t.Fatalf("run: %v", err)
	}
	var v int
	if err := db.QueryRow("SELECT MAX(version) FROM schema_version").Scan(&v); err != nil {
		t.Fatalf("schema_version: %v", err)
	}
	if v != 1 {
		t.Errorf("version = %d, want 1", v)
	}
	if _, err := db.Exec("INSERT INTO foo(id) VALUES (1)"); err != nil {
		t.Errorf("insert into foo: %v", err)
	}
}

func TestRunMigrations_Idempotent(t *testing.T) {
	db := openMemory(t)
	fsys := fstest.MapFS{
		"migrations/0001_test.sql": &fstest.MapFile{
			Data: []byte(`CREATE TABLE foo (id INTEGER PRIMARY KEY)`),
		},
	}
	if err := runMigrations(db, fsys); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if err := runMigrations(db, fsys); err != nil {
		t.Fatalf("second run: %v", err)
	}
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM schema_version").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("schema_version rows = %d, want 1", n)
	}
}

func TestRunMigrations_MalformedSQL_ReturnsError(t *testing.T) {
	db := openMemory(t)
	fsys := fstest.MapFS{
		"migrations/0001_bad.sql": &fstest.MapFile{
			Data: []byte(`THIS IS NOT VALID SQL;`),
		},
	}
	err := runMigrations(db, fsys)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	var v int
	if dbErr := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&v); dbErr != nil {
		t.Fatalf("query: %v", dbErr)
	}
	if v != 0 {
		t.Errorf("version = %d after failed migration, want 0", v)
	}
}

func TestRunMigrations_OrdersByVersion(t *testing.T) {
	db := openMemory(t)
	fsys := fstest.MapFS{
		"migrations/0002_second.sql": &fstest.MapFile{
			Data: []byte(`CREATE TABLE bar (id INTEGER PRIMARY KEY)`),
		},
		"migrations/0001_first.sql": &fstest.MapFile{
			Data: []byte(`CREATE TABLE foo (id INTEGER PRIMARY KEY)`),
		},
	}
	if err := runMigrations(db, fsys); err != nil {
		t.Fatalf("run: %v", err)
	}
	var v int
	if err := db.QueryRow("SELECT MAX(version) FROM schema_version").Scan(&v); err != nil {
		t.Fatalf("query: %v", err)
	}
	if v != 2 {
		t.Errorf("version = %d, want 2", v)
	}
}

func TestPathBase(t *testing.T) {
	cases := map[string]string{
		"a/b/c.sql":            "c.sql",
		"plain.sql":            "plain.sql",
		"migrations/0001_x.sql": "0001_x.sql",
	}
	for in, want := range cases {
		if got := pathBase(in); got != want {
			t.Errorf("pathBase(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseLeadingVersion(t *testing.T) {
	cases := []struct {
		in   string
		ok   bool
		want int
	}{
		{"0001_init.sql", true, 1},
		{"0042_seed.sql", true, 42},
		{"no_underscore_at_start", false, 0}, // "no" before underscore can't parse as int
		{"_starts_with_underscore.sql", false, 0},
		{"plain.sql", false, 0},
	}
	for _, c := range cases {
		got, err := parseLeadingVersion(c.in)
		if c.ok && err != nil {
			t.Errorf("parseLeadingVersion(%q) unexpected error: %v", c.in, err)
		}
		if !c.ok && err == nil {
			t.Errorf("parseLeadingVersion(%q) expected error, got %d", c.in, got)
		}
		if c.ok && got != c.want {
			t.Errorf("parseLeadingVersion(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestRunMigrations_EmbeddedInitial(t *testing.T) {
	db := openMemory(t)
	sub, err := fs.Sub(migrationsFS, ".")
	if err != nil {
		t.Fatalf("fs.Sub: %v", err)
	}
	if err := runMigrations(db, sub); err != nil {
		t.Fatalf("run: %v", err)
	}
	var v int
	if err := db.QueryRow("SELECT MAX(version) FROM schema_version").Scan(&v); err != nil {
		t.Fatalf("schema_version: %v", err)
	}
	if v != 2 {
		t.Errorf("version = %d, want 2", v)
	}

	for _, table := range []string{"events", "sessions", "prompts", "metric_snapshots"} {
		var n int
		err := db.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table,
		).Scan(&n)
		if err != nil {
			t.Fatalf("check %s: %v", table, err)
		}
		if n != 1 {
			t.Errorf("table %s: count=%d, want 1", table, n)
		}
	}

	wantIndexes := []string{
		"idx_events_session_ts", "idx_events_prompt", "idx_events_name_ts",
		"idx_sessions_started", "idx_sessions_project_started",
		"idx_sessions_last_seen",
		"idx_prompts_session_started",
	}
	for _, ix := range wantIndexes {
		var n int
		err := db.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?`, ix,
		).Scan(&n)
		if err != nil {
			t.Fatalf("check %s: %v", ix, err)
		}
		if n != 1 {
			t.Errorf("index %s: count=%d, want 1", ix, n)
		}
	}
}
