package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

const envelopeVersion = "v1"

type Keyring struct {
	active string
	keys   map[string]cipher.AEAD
}

func NewKeyring(raw string) (Keyring, error) {
	result := Keyring{keys: make(map[string]cipher.AEAD)}
	for index, entry := range strings.Split(raw, ",") {
		parts := strings.Split(strings.TrimSpace(entry), ":")
		if len(parts) != 2 || !validKeyID(parts[0]) {
			return Keyring{}, errors.New("cipher keyring entries must use kid:base64url-key format")
		}
		key, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil || len(key) != 32 {
			return Keyring{}, fmt.Errorf("cipher key %q must be an unpadded base64url-encoded 32-byte key", parts[0])
		}
		if _, exists := result.keys[parts[0]]; exists {
			return Keyring{}, fmt.Errorf("cipher key id %q is duplicated", parts[0])
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			return Keyring{}, fmt.Errorf("create cipher key %q: %w", parts[0], err)
		}
		aead, err := cipher.NewGCM(block)
		if err != nil {
			return Keyring{}, fmt.Errorf("create GCM key %q: %w", parts[0], err)
		}
		if index == 0 {
			result.active = parts[0]
		}
		result.keys[parts[0]] = aead
	}
	if result.active == "" {
		return Keyring{}, errors.New("cipher keyring must contain at least one key")
	}
	return result, nil
}

func (k Keyring) Seal(purpose, subject, value string) ([]byte, error) {
	if purpose == "" || subject == "" {
		return nil, errors.New("cipher purpose and subject are required")
	}
	aead, ok := k.keys[k.active]
	if !ok {
		return nil, errors.New("cipher keyring has no active key")
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("read cipher nonce: %w", err)
	}
	payload := aead.Seal(nonce, nonce, []byte(value), additionalData(k.active, purpose, subject))
	return []byte(envelopeVersion + ":" + k.active + ":" + base64.RawURLEncoding.EncodeToString(payload)), nil
}

// Open returns whether the ciphertext already uses the active key. Callers can
// lazily rotate durable values without exposing plaintext outside the service.
func (k Keyring) Open(purpose, subject string, value []byte) (string, bool, error) {
	parts := strings.Split(string(value), ":")
	if len(parts) != 3 || parts[0] != envelopeVersion {
		return "", false, errors.New("ciphertext envelope is invalid")
	}
	aead, ok := k.keys[parts[1]]
	if !ok {
		return "", false, fmt.Errorf("cipher key id %q is unavailable", parts[1])
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(payload) < aead.NonceSize()+aead.Overhead() {
		return "", false, errors.New("ciphertext payload is invalid")
	}
	plaintext, err := aead.Open(nil, payload[:aead.NonceSize()], payload[aead.NonceSize():], additionalData(parts[1], purpose, subject))
	if err != nil {
		return "", false, errors.New("ciphertext authentication failed")
	}
	return string(plaintext), parts[1] == k.active, nil
}

func additionalData(keyID, purpose, subject string) []byte {
	return []byte(strings.Join([]string{envelopeVersion, keyID, purpose, subject}, "\x00"))
}

func validKeyID(value string) bool {
	if len(value) < 1 || len(value) > 32 {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') &&
			(char < '0' || char > '9') && char != '-' && char != '_' {
			return false
		}
	}
	return true
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

func (t Tokens) Derive(purpose, value string) string {
	mac := hmac.New(sha256.New, t.key)
	_, _ = mac.Write([]byte(purpose))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (t Tokens) MatchesDerived(raw, purpose, value string) bool {
	return hmac.Equal([]byte(raw), []byte(t.Derive(purpose, value)))
}
