package shortener

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/phngkhuongduy/shortlink/internal/repository"
)

// Service-level errors. The HTTP layer maps these to status codes.
var (
	// ErrInvalidURL is returned when the input is not an acceptable URL.
	ErrInvalidURL = errors.New("invalid url")
	// ErrNotFound is returned when a code has no mapping.
	ErrNotFound = errors.New("short url not found")
	// ErrGenerateFailed is returned when a unique code could not be produced
	// within the retry budget.
	ErrGenerateFailed = errors.New("could not generate a unique code")
)

// maxURLLength bounds stored URLs to prevent resource-exhaustion abuse.
const maxURLLength = 2048

// allowedSchemes restricts what we shorten. Blocking non-http(s) schemes
// prevents the service from being used to obfuscate javascript:, data: or
// file: payloads (see README threat model).
var allowedSchemes = map[string]bool{"http": true, "https": true}

// Service implements URL encoding and decoding.
type Service struct {
	repo       repository.Repository
	gen        Generator
	maxRetries int
}

// NewService wires a Service. maxRetries bounds collision retries; <=0 uses 5.
func NewService(repo repository.Repository, gen Generator, maxRetries int) *Service {
	if maxRetries <= 0 {
		maxRetries = 5
	}
	return &Service{repo: repo, gen: gen, maxRetries: maxRetries}
}

// Encode validates rawURL, generates a unique short code, persists the
// mapping, and returns the code. It retries on the (rare) code collision.
func (s *Service) Encode(ctx context.Context, rawURL string) (string, error) {
	if err := validateURL(rawURL); err != nil {
		return "", err
	}
	for attempt := 0; attempt < s.maxRetries; attempt++ {
		code, err := s.gen.Generate()
		if err != nil {
			return "", fmt.Errorf("generate code: %w", err)
		}
		err = s.repo.Save(ctx, code, rawURL)
		switch {
		case err == nil:
			return code, nil
		case errors.Is(err, repository.ErrCodeExists):
			continue // collision — try a new code
		default:
			return "", fmt.Errorf("save mapping: %w", err)
		}
	}
	return "", ErrGenerateFailed
}

// Decode resolves a short code (or full short URL) back to its long URL.
func (s *Service) Decode(ctx context.Context, codeOrURL string) (string, error) {
	code, err := extractCode(codeOrURL)
	if err != nil {
		return "", err
	}
	longURL, err := s.repo.Get(ctx, code)
	if errors.Is(err, repository.ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get mapping: %w", err)
	}
	return longURL, nil
}

// validateURL enforces the rules described in the threat model.
func validateURL(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return fmt.Errorf("%w: empty url", ErrInvalidURL)
	}
	if len(rawURL) > maxURLLength {
		return fmt.Errorf("%w: url exceeds %d characters", ErrInvalidURL, maxURLLength)
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}
	if !allowedSchemes[strings.ToLower(u.Scheme)] {
		return fmt.Errorf("%w: scheme %q not allowed", ErrInvalidURL, u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("%w: missing host", ErrInvalidURL)
	}
	return nil
}

// extractCode accepts either a bare code ("GeAi9K") or a full short URL
// ("http://host/GeAi9K") and returns the code. It validates the code's
// character set to avoid pointless storage lookups on malformed input.
func extractCode(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("%w: empty code", ErrInvalidURL)
	}
	code := input
	if strings.Contains(input, "/") {
		// Treat as a URL/path and take the last non-empty segment.
		trimmed := strings.TrimRight(input, "/")
		idx := strings.LastIndex(trimmed, "/")
		code = trimmed[idx+1:]
	}
	if code == "" || !isBase62(code) {
		return "", fmt.Errorf("%w: malformed code", ErrInvalidURL)
	}
	return code, nil
}

func isBase62(s string) bool {
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}
