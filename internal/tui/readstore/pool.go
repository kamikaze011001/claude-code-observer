package readstore

import (
	"database/sql"
	"fmt"
	"net/url"

	_ "modernc.org/sqlite"
)

// OpenRO opens a read-only pool against the SQLite file at path.
//
// Note: open succeeds even when the file is missing — modernc.org/sqlite
// defers the error to the first query. Callers must treat first-query
// errors as "DB not ready" rather than fatal.
func OpenRO(path string) (*sql.DB, error) {
	q := url.Values{}
	q.Set("mode", "ro")
	q.Set("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "busy_timeout(2000)")
	q.Add("_pragma", "query_only(1)")
	dsn := fmt.Sprintf("file:%s?%s", path, q.Encode())

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open ro sqlite: %w", err)
	}
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(0)
	return db, nil
}
