package repository

import (
	"context"
	"sync"
)

// Memory is an in-memory Repository implementation.
//
// It is used in tests (fast, no external dependency, safe under `go test
// -race`) and as a fallback when no database is configured. Data is not
// persisted across restarts — use the SQLite implementation for that.
type Memory struct {
	mu   sync.RWMutex
	data map[string]string
}

// NewMemory returns an empty in-memory repository.
func NewMemory() *Memory {
	return &Memory{data: make(map[string]string)}
}

func (m *Memory) Save(_ context.Context, code, longURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.data[code]; ok {
		return ErrCodeExists
	}
	m.data[code] = longURL
	return nil
}

func (m *Memory) Get(_ context.Context, code string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	longURL, ok := m.data[code]
	if !ok {
		return "", ErrNotFound
	}
	return longURL, nil
}

func (m *Memory) Close() error { return nil }
