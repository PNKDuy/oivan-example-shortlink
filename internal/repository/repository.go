package repository

import (
	"context"
	"errors"
)

// Sentinel errors returned by Repository implementations.
var (
	// ErrNotFound is returned by Get when the code does not exist.
	ErrNotFound = errors.New("repository: code not found")
	// ErrCodeExists is returned by Save when the code already exists.
	// The service layer uses this to retry with a freshly generated code.
	ErrCodeExists = errors.New("repository: code already exists")
)

// Repository persists the mapping between a short code and its long URL.
//
// It is the single seam that decouples business logic from storage: the
// service depends only on this interface, so swapping SQLite for MySQL,
// Postgres, or an in-memory store (used in tests) requires no change above
// this layer.
type Repository interface {
	// Save stores the code -> longURL mapping. It must return ErrCodeExists
	// if the code is already taken, so the caller can retry.
	Save(ctx context.Context, code, longURL string) error
	// Get returns the long URL for a code, or ErrNotFound if absent.
	Get(ctx context.Context, code string) (string, error)
	// Close releases any underlying resources (e.g. the database handle).
	Close() error
}
