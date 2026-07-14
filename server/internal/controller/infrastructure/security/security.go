package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

type Cipher struct {
	aead cipher.AEAD
}

func NewCipher(key string) (Cipher, error) {
	digest := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(digest[:])
	if err != nil {
		return Cipher{}, fmt.Errorf("create oauth state cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return Cipher{}, fmt.Errorf("create oauth state GCM: %w", err)
	}
	return Cipher{aead: aead}, nil
}

func (c Cipher) Seal(value string) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("read cipher nonce: %w", err)
	}
	return c.aead.Seal(nonce, nonce, []byte(value), nil), nil
}

func (c Cipher) Open(value []byte) (string, error) {
	nonceSize := c.aead.NonceSize()
	if len(value) < nonceSize {
		return "", fmt.Errorf("oauth state ciphertext is truncated")
	}
	plaintext, err := c.aead.Open(nil, value[:nonceSize], value[nonceSize:], nil)
	if err != nil {
		return "", fmt.Errorf("open oauth state ciphertext: %w", err)
	}
	return string(plaintext), nil
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
