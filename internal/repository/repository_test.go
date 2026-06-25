package repository

import (
	"context"
	"errors"
	"testing"
)

// runConformance exercises the Repository contract. Both Memory and SQLite
// run against it so the two implementations stay behaviourally identical.
func runConformance(t *testing.T, repo Repository) {
	t.Helper()
	ctx := context.Background()

	// Save then Get round-trips.
	if err := repo.Save(ctx, "abc", "https://example.com"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := repo.Get(ctx, "abc")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "https://example.com" {
		t.Fatalf("Get = %q, want https://example.com", got)
	}

	// Missing code returns ErrNotFound.
	if _, err := repo.Get(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(missing) err = %v, want ErrNotFound", err)
	}

	// Duplicate code returns ErrCodeExists.
	if err := repo.Save(ctx, "abc", "https://other.com"); !errors.Is(err, ErrCodeExists) {
		t.Fatalf("Save(dup) err = %v, want ErrCodeExists", err)
	}
}

func TestMemory_Conformance(t *testing.T) {
	repo := NewMemory()
	defer repo.Close()
	runConformance(t, repo)
}
