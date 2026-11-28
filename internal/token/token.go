package token

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// New returns a cryptographically random token encoded as hexadecimal.
// length is the number of random bytes, so the returned string is twice as long.
func New(length int) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("token length must be positive")
	}
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
