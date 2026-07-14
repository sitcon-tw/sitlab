package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

type PasswordHasher struct {
	cost int
}

func NewPasswordHasher(cost int) PasswordHasher {
	if cost == 0 {
		cost = bcrypt.DefaultCost
	}
	return PasswordHasher{cost: cost}
}

func (h PasswordHasher) Hash(password string) (string, error) {
	digest, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", fmt.Errorf("bcrypt password: %w", err)
	}
	return string(digest), nil
}

func (h PasswordHasher) Compare(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

type Tokens struct {
	key []byte
}

func NewTokens(key string) Tokens { return Tokens{key: []byte(key)} }

func (t Tokens) New() (string, []byte, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", nil, fmt.Errorf("read secure randomness: %w", err)
	}
	raw := base64.RawURLEncoding.EncodeToString(secret)
	return raw, t.Digest(raw), nil
}

func (t Tokens) Digest(raw string) []byte {
	mac := hmac.New(sha256.New, t.key)
	_, _ = mac.Write([]byte(raw))
	return mac.Sum(nil)
}

func (t Tokens) Matches(raw string, expected []byte) bool {
	return hmac.Equal(t.Digest(raw), expected)
}
