package shortener

import (
	"regexp"
	"testing"
)

var base62Re = regexp.MustCompile(`^[A-Za-z0-9]+$`)

func TestRandomGenerator_LengthAndCharset(t *testing.T) {
	g := NewRandomGenerator(7)
	for i := 0; i < 1000; i++ {
		code, err := g.Generate()
		if err != nil {
			t.Fatalf("Generate() error: %v", err)
		}
		if len(code) != 7 {
			t.Fatalf("len(code) = %d, want 7", len(code))
		}
		if !base62Re.MatchString(code) {
			t.Fatalf("code %q is not base62", code)
		}
	}
}

func TestRandomGenerator_DefaultLength(t *testing.T) {
	g := NewRandomGenerator(0) // should fall back to 7
	code, err := g.Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if len(code) != 7 {
		t.Fatalf("default len = %d, want 7", len(code))
	}
}

func TestRandomGenerator_Uniqueness(t *testing.T) {
	// Sanity check: a large sample should have very few (ideally zero)
	// duplicates for a 7-char space. This guards against a broken source.
	g := NewRandomGenerator(7)
	seen := make(map[string]struct{}, 10000)
	dups := 0
	for i := 0; i < 10000; i++ {
		code, err := g.Generate()
		if err != nil {
			t.Fatalf("Generate() error: %v", err)
		}
		if _, ok := seen[code]; ok {
			dups++
		}
		seen[code] = struct{}{}
	}
	if dups > 1 {
		t.Fatalf("got %d duplicates in 10k samples, generator may be weak", dups)
	}
}
