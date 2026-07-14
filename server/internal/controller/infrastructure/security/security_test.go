package security

import "testing"

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
