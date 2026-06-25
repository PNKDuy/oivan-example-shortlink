package shortener

import (
	"crypto/rand"
	"math/big"
)

// base62Alphabet is the set of characters used for short codes.
// 62 characters => 62^n possibilities for a code of length n.
const base62Alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

// Generator produces random short codes.
type Generator interface {
	Generate() (string, error)
}

// RandomGenerator generates codes of a fixed length using crypto/rand.
//
// We deliberately use a cryptographically secure source rather than math/rand
// so codes cannot be predicted or enumerated by an attacker (see README:
// "Code enumeration" threat). Uniqueness is enforced by the repository, not
// here — the service retries on the rare collision.
type RandomGenerator struct {
	length int
}

// NewRandomGenerator returns a generator producing codes of the given length.
// A length <= 0 falls back to the default of 7 characters (62^7 ≈ 3.5e12).
func NewRandomGenerator(length int) *RandomGenerator {
	if length <= 0 {
		length = 7
	}
	return &RandomGenerator{length: length}
}

// Generate returns a random base62 string of the configured length.
func (g *RandomGenerator) Generate() (string, error) {
	max := big.NewInt(int64(len(base62Alphabet)))
	b := make([]byte, g.length)
	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = base62Alphabet[n.Int64()]
	}
	return string(b), nil
}
