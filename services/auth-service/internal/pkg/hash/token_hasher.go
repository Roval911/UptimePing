package hash

import (
	"crypto/sha256"
	"encoding/base64"
)

// TokenHasher хеширует токены с использованием SHA256
type TokenHasher struct{}

// NewTokenHasher создает новый экземпляр TokenHasher
func NewTokenHasher() *TokenHasher {
	return &TokenHasher{}
}

// Hash хеширует токен с использованием SHA256
func (h *TokenHasher) Hash(token string) (string, error) {
	hash := sha256.Sum256([]byte(token))
	return base64.StdEncoding.EncodeToString(hash[:]), nil
}

// Verify проверяет токен против хеша
func (h *TokenHasher) Verify(token, hash string) bool {
	computedHash, err := h.Hash(token)
	if err != nil {
		return false
	}
	return computedHash == hash
}
