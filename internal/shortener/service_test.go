package shortener

import (
	"context"
	"errors"
	"testing"

	"github.com/phngkhuongduy/shortlink/internal/repository"
)

// stubGenerator returns predetermined codes in order, repeating the last one
// once exhausted. It lets tests force collisions deterministically.
type stubGenerator struct {
	codes []string
	i     int
}

func (s *stubGenerator) Generate() (string, error) {
	if s.i >= len(s.codes) {
		return s.codes[len(s.codes)-1], nil
	}
	c := s.codes[s.i]
	s.i++
	return c, nil
}

func newService(t *testing.T, gen Generator) (*Service, repository.Repository) {
	t.Helper()
	repo := repository.NewMemory()
	return NewService(repo, gen, 5), repo
}

func TestService_EncodeDecode_RoundTrip(t *testing.T) {
	svc, _ := newService(t, NewRandomGenerator(7))
	ctx := context.Background()

	urls := []string{
		"https://codesubmit.io/library/react",
		"http://example.com",
		"https://example.com/path?q=1&x=2#frag",
	}
	for _, u := range urls {
		code, err := svc.Encode(ctx, u)
		if err != nil {
			t.Fatalf("Encode(%q) error: %v", u, err)
		}
		got, err := svc.Decode(ctx, code)
		if err != nil {
			t.Fatalf("Decode(%q) error: %v", code, err)
		}
		if got != u {
			t.Fatalf("round trip = %q, want %q", got, u)
		}
	}
}

func TestService_Encode_InvalidURL(t *testing.T) {
	svc, _ := newService(t, NewRandomGenerator(7))
	ctx := context.Background()

	cases := []struct {
		name string
		url  string
	}{
		{"empty", ""},
		{"whitespace", "   "},
		{"no scheme", "example.com"},
		{"javascript scheme", "javascript:alert(1)"},
		{"data scheme", "data:text/html,<script>alert(1)</script>"},
		{"file scheme", "file:///etc/passwd"},
		{"missing host", "http://"},
		{"too long", "https://example.com/" + string(make([]byte, 3000))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Encode(ctx, tc.url)
			if !errors.Is(err, ErrInvalidURL) {
				t.Fatalf("Encode(%q) err = %v, want ErrInvalidURL", tc.url, err)
			}
		})
	}
}

func TestService_Encode_RetriesOnCollision(t *testing.T) {
	// "dup" is already taken; generator yields it first, then "fresh".
	repo := repository.NewMemory()
	if err := repo.Save(context.Background(), "dup", "https://taken.example"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	gen := &stubGenerator{codes: []string{"dup", "fresh"}}
	svc := NewService(repo, gen, 5)

	code, err := svc.Encode(context.Background(), "https://new.example")
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}
	if code != "fresh" {
		t.Fatalf("code = %q, want fresh (should have retried past collision)", code)
	}
}

func TestService_Encode_ExhaustsRetries(t *testing.T) {
	repo := repository.NewMemory()
	if err := repo.Save(context.Background(), "dup", "https://taken.example"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	gen := &stubGenerator{codes: []string{"dup"}} // always collides
	svc := NewService(repo, gen, 3)

	_, err := svc.Encode(context.Background(), "https://new.example")
	if !errors.Is(err, ErrGenerateFailed) {
		t.Fatalf("err = %v, want ErrGenerateFailed", err)
	}
}

func TestService_Decode_NotFound(t *testing.T) {
	svc, _ := newService(t, NewRandomGenerator(7))
	_, err := svc.Decode(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestService_Decode_AcceptsCodeOrFullURL(t *testing.T) {
	svc, _ := newService(t, &stubGenerator{codes: []string{"abc123"}})
	ctx := context.Background()
	const long = "https://example.com/page"
	if _, err := svc.Encode(ctx, long); err != nil {
		t.Fatalf("Encode: %v", err)
	}

	cases := []string{
		"abc123",
		"http://localhost:8080/abc123",
		"https://your.domain/abc123/",
	}
	for _, in := range cases {
		got, err := svc.Decode(ctx, in)
		if err != nil {
			t.Fatalf("Decode(%q) error: %v", in, err)
		}
		if got != long {
			t.Fatalf("Decode(%q) = %q, want %q", in, got, long)
		}
	}
}

func TestService_Decode_MalformedCode(t *testing.T) {
	svc, _ := newService(t, NewRandomGenerator(7))
	cases := []string{"", "  ", "bad code!", "has/two/segments/!!"}
	for _, in := range cases {
		_, err := svc.Decode(context.Background(), in)
		if !errors.Is(err, ErrInvalidURL) {
			t.Fatalf("Decode(%q) err = %v, want ErrInvalidURL", in, err)
		}
	}
}
