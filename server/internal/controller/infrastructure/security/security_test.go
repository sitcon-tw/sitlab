package security

import "testing"

func TestCipherRoundTripAndAuthentication(t *testing.T) {
	t.Parallel()
	cipher, err := NewCipher("test-only-key-that-is-long-enough-for-aes-gcm")
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := cipher.Seal("pkce-verifier")
	if err != nil {
		t.Fatal(err)
	}
	opened, err := cipher.Open(sealed)
	if err != nil || opened != "pkce-verifier" {
		t.Fatalf("Open() = %q, %v", opened, err)
	}
	sealed[len(sealed)-1] ^= 0xff
	if _, err := cipher.Open(sealed); err == nil {
		t.Fatal("tampered ciphertext was accepted")
	}
}

func TestTokensAreOpaqueAndKeyed(t *testing.T) {
	t.Parallel()
	tokens := NewTokens("01234567890123456789012345678901")
	raw, digest, err := tokens.New()
	if err != nil {
		t.Fatal(err)
	}
	if raw == "" || string(digest) == raw || !tokens.Matches(raw, digest) {
		t.Fatal("token invariant failed")
	}
	if NewTokens("another-key-which-is-long-enough-000").Matches(raw, digest) {
		t.Fatal("digest must be keyed")
	}
}
