// Package readstore opens a read-only connection pool to the SQLite file
// owned by the daemon's writer pool. WAL mode keeps reads and writes
// non-blocking. mode=ro and the query_only PRAGMA enforce safety.
package readstore
