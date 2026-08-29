package security

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func testKey(seed byte) string {
	return base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{seed}, 32))
}

func TestKeyringRoundTripRotationAndAuthentication(t *testing.T) {
	t.Parallel()
	oldRing, err := NewKeyring("old:" + testKey(1))
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := oldRing.Seal("gitlab-access-token", "user-1", "secret")
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := NewKeyring("current:" + testKey(2) + ",old:" + testKey(1))
	if err != nil {
		t.Fatal(err)
	}
	opened, current, err := rotated.Open("gitlab-access-token", "user-1", sealed)
	if err != nil || opened != "secret" || current {
		t.Fatalf("Open() = %q, %v, current = %t", opened, err, current)
	}
	if _, _, err := rotated.Open("gitlab-refresh-token", "user-1", sealed); err == nil {
		t.Fatal("Open() accepted ciphertext under another purpose")
	}
	if _, _, err := rotated.Open("gitlab-access-token", "user-2", sealed); err == nil {
		t.Fatal("Open() accepted ciphertext for another subject")
	}
	sealed[len(sealed)-1] ^= 0xff
	if _, _, err := rotated.Open("gitlab-access-token", "user-1", sealed); err == nil {
		t.Fatal("tampered ciphertext was accepted")
	}
}

func TestKeyringRejectsMalformedConfiguration(t *testing.T) {
	t.Parallel()
	tests := []string{"", "missing-colon", "bad kid:" + testKey(1), "kid:not-base64", "kid:" + testKey(1) + ",kid:" + testKey(2)}
	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			if _, err := NewKeyring(raw); err == nil {
				t.Fatalf("NewKeyring(%q) accepted malformed configuration", raw)
			}
		})
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
	derived := tokens.Derive("csrf", "session-id")
	if derived == "" || derived != tokens.Derive("csrf", "session-id") || !tokens.MatchesDerived(derived, "csrf", "session-id") {
		t.Fatal("derived token must be stable and verifiable")
	}
	if tokens.MatchesDerived(derived, "oauth", "session-id") || tokens.MatchesDerived(derived, "csrf", "another-session") {
		t.Fatal("derived tokens must be purpose- and session-bound")
	}
}
