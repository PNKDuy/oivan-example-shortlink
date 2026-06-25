package repository

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
)

func TestSQLite_Conformance(t *testing.T) {
	repo, err := NewSQLite(filepath.Join(t.TempDir(), "conf.db"))
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	defer repo.Close()
	runConformance(t, repo)
}

// TestSQLite_PersistsAcrossRestart proves the core requirement: a mapping
// written by one instance is readable by a fresh instance opening the same
// file — i.e. data survives a process restart.
func TestSQLite_PersistsAcrossRestart(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "restart.db")
	ctx := context.Background()

	first, err := NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("open first: %v", err)
	}
	if err := first.Save(ctx, "keep", "https://persisted.example"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := first.Close(); err != nil { // simulate shutdown
		t.Fatalf("Close: %v", err)
	}

	second, err := NewSQLite(dbPath) // simulate restart
	if err != nil {
		t.Fatalf("open second: %v", err)
	}
	defer second.Close()

	got, err := second.Get(ctx, "keep")
	if err != nil {
		t.Fatalf("Get after restart: %v", err)
	}
	if got != "https://persisted.example" {
		t.Fatalf("after restart = %q, want https://persisted.example", got)
	}
}

// TestSQLite_ConcurrentSaves runs under `go test -race` to verify the store is
// safe under concurrent writers and never assigns one code to two URLs.
func TestSQLite_ConcurrentSaves(t *testing.T) {
	repo, err := NewSQLite(filepath.Join(t.TempDir(), "conc.db"))
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	defer repo.Close()
	ctx := context.Background()

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			code := fmt.Sprintf("code%03d", i)
			if err := repo.Save(ctx, code, fmt.Sprintf("https://example.com/%d", i)); err != nil {
				t.Errorf("Save(%s): %v", code, err)
			}
		}(i)
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		code := fmt.Sprintf("code%03d", i)
		got, err := repo.Get(ctx, code)
		if err != nil {
			t.Fatalf("Get(%s): %v", code, err)
		}
		if want := fmt.Sprintf("https://example.com/%d", i); got != want {
			t.Fatalf("Get(%s) = %q, want %q", code, got, want)
		}
	}
}
