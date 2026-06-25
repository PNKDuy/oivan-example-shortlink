package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// SQLite is a Repository backed by an SQLite database file.
//
// We use the pure-Go driver (modernc.org/sqlite) so the project builds and
// cross-compiles without CGO / a C toolchain. The file-based store satisfies
// the "decode after restart" requirement: reopening the same file restores all
// mappings.
type SQLite struct {
	db *sql.DB
}

// NewSQLite opens (or creates) the database at path and ensures the schema
// exists. path is an SQLite file path, e.g. "shortlink.db".
//
// PRAGMAs are set via the DSN so every pooled connection inherits them — in
// particular busy_timeout, without which concurrent writers fail immediately
// with SQLITE_BUSY instead of waiting. WAL mode allows concurrent readers
// alongside a single writer. SQLite permits only one writer at a time, so we
// cap the pool to one connection to serialize writes deterministically; this
// is the well-known recommendation for embedded SQLite and is exercised by
// TestSQLite_ConcurrentSaves under `go test -race`.
func NewSQLite(path string) (*SQLite, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(on)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	const schema = `
CREATE TABLE IF NOT EXISTS urls (
    code       TEXT PRIMARY KEY,
    long_url   TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);`
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}
	return &SQLite{db: db}, nil
}

func (s *SQLite) Save(ctx context.Context, code, longURL string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO urls (code, long_url) VALUES (?, ?)`, code, longURL)
	if err != nil {
		// Translate a PRIMARY KEY violation into ErrCodeExists so the service
		// can retry with a new code. This is how collisions are handled.
		var serr *sqlite.Error
		if errors.As(err, &serr) {
			switch serr.Code() {
			case sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY, sqlite3.SQLITE_CONSTRAINT_UNIQUE:
				return ErrCodeExists
			}
		}
		return fmt.Errorf("insert url: %w", err)
	}
	return nil
}

func (s *SQLite) Get(ctx context.Context, code string) (string, error) {
	var longURL string
	err := s.db.QueryRowContext(ctx,
		`SELECT long_url FROM urls WHERE code = ?`, code).Scan(&longURL)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("query url: %w", err)
	}
	return longURL, nil
}

func (s *SQLite) Close() error { return s.db.Close() }
