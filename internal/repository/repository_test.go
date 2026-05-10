package repository

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpen_CreatesDatabaseFile(t *testing.T) {
	home := t.TempDir()
	repo, err := Open(home)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer repo.Close()
	want := filepath.Join(home, "db.sqlite")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("expected %s to exist: %v", want, err)
	}
}

func TestOpen_AppliesMigrations(t *testing.T) {
	repo, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer repo.Close()
	var v int
	err = repo.DB().QueryRow("SELECT MAX(version) FROM schema_version").Scan(&v)
	if err != nil {
		t.Fatalf("schema_version: %v", err)
	}
	if v != 1 {
		t.Errorf("schema_version = %d, want 1", v)
	}
}

func TestOpen_Idempotent(t *testing.T) {
	home := t.TempDir()

	repo1, err := Open(home)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	repo1.Close()

	repo2, err := Open(home)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer repo2.Close()

	var n int
	if err := repo2.DB().QueryRow("SELECT COUNT(*) FROM schema_version").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("schema_version rows = %d, want 1", n)
	}
}

func TestOpen_WALModeAndForeignKeys(t *testing.T) {
	repo, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer repo.Close()

	var mode string
	if err := repo.DB().QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %s, want wal", mode)
	}

	var fk int
	if err := repo.DB().QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1", fk)
	}
}

func TestOpen_CreatesHomeDirIfMissing(t *testing.T) {
	parent := t.TempDir()
	home := filepath.Join(parent, "nested", "home")
	repo, err := Open(home)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer repo.Close()
	if _, err := os.Stat(home); err != nil {
		t.Errorf("home dir not created: %v", err)
	}
}
