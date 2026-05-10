package readstore_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/kamikaze011001/claude-code-observer/internal/repository"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
)

func TestOpenRO_OpensExistingDB(t *testing.T) {
	home := t.TempDir()
	repo, err := repository.Open(home)
	if err != nil {
		t.Fatalf("repository.Open: %v", err)
	}
	if err := repo.Close(); err != nil {
		t.Fatalf("repo.Close: %v", err)
	}

	dbPath := filepath.Join(home, "db.sqlite")
	pool, err := readstore.OpenRO(dbPath)
	if err != nil {
		t.Fatalf("OpenRO: %v", err)
	}
	defer pool.Close()

	var n int
	if err := pool.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM sessions").Scan(&n); err != nil {
		t.Fatalf("SELECT: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 sessions, got %d", n)
	}
}

func TestOpenRO_RejectsWrites(t *testing.T) {
	home := t.TempDir()
	repo, _ := repository.Open(home)
	repo.Close()

	pool, err := readstore.OpenRO(filepath.Join(home, "db.sqlite"))
	if err != nil {
		t.Fatalf("OpenRO: %v", err)
	}
	defer pool.Close()

	_, err = pool.ExecContext(context.Background(),
		"INSERT INTO sessions(session_id, started_at, last_seen_at) VALUES('x', 0, 0)")
	if err == nil {
		t.Fatalf("expected write to fail under query_only/mode=ro")
	}
}

func TestOpenRO_FileMissing(t *testing.T) {
	pool, err := readstore.OpenRO("/nonexistent/db.sqlite")
	if err != nil {
		return
	}
	defer pool.Close()
	var n int
	if err := pool.QueryRowContext(context.Background(),
		"SELECT 1").Scan(&n); err == nil {
		t.Fatalf("expected error querying missing DB")
	}
}
