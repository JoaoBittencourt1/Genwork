package auth

import (
	"strings"
	"testing"
	"time"
)

func newTestKey(t *testing.T) SigningKey {
	t.Helper()
	key, err := ParseSigningKey(GenerateSigningKeySeed())
	if err != nil {
		t.Fatalf("parse key: %v", err)
	}
	return key
}

func TestParseSigningKeyRejectsInvalidSeeds(t *testing.T) {
	for _, seed := range []string{"", "not base64!", "c2hvcnQ="} {
		if _, err := ParseSigningKey(seed); err == nil {
			t.Errorf("expected %q to be rejected", seed)
		}
	}
}

func TestIssueAndVerifyAccessToken(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	issuer := NewAccessTokenIssuer(newTestKey(t), "http://localhost:8485", func() time.Time { return now })

	token, expiresAt, err := issuer.Issue("user-token", "session-token")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if !expiresAt.Equal(now.Add(15 * time.Minute)) {
		t.Errorf("expiresAt = %s", expiresAt)
	}
	claims, err := issuer.Verify(token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.Subject != "user-token" || claims.SessionToken != "session-token" {
		t.Errorf("claims = %+v", claims)
	}
}

func TestVerifyRejectsInvalidTokens(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	key := newTestKey(t)
	issuer := NewAccessTokenIssuer(key, "http://localhost:8485", clock)
	valid, _, _ := issuer.Issue("user-token", "session-token")

	otherKeyToken, _, _ := NewAccessTokenIssuer(newTestKey(t), "http://localhost:8485", clock).Issue("user-token", "session-token")
	otherIssuerToken, _, _ := NewAccessTokenIssuer(key, "http://elsewhere", clock).Issue("user-token", "session-token")
	tampered := valid[:len(valid)-4] + strings.Repeat("A", 4)

	cases := map[string]string{
		"signed by another key": otherKeyToken,
		"another issuer":        otherIssuerToken,
		"tampered signature":    tampered,
		"garbage":               "abc.def.ghi",
	}
	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := issuer.Verify(token); err == nil {
				t.Fatal("expected verification to fail")
			}
		})
	}

	t.Run("expired", func(t *testing.T) {
		later := NewAccessTokenIssuer(key, "http://localhost:8485", func() time.Time { return now.Add(16 * time.Minute) })
		if _, err := later.Verify(valid); err == nil {
			t.Fatal("expected expired token to fail")
		}
	})
}

func TestJWKSetPublishesEd25519PublicKey(t *testing.T) {
	set := newTestKey(t).JWKSet()
	if len(set.Keys) != 1 {
		t.Fatalf("keys = %d", len(set.Keys))
	}
	jwk := set.Keys[0]
	if jwk.KeyType != "OKP" || jwk.Curve != "Ed25519" || jwk.Algorithm != "EdDSA" || jwk.KeyID == "" || jwk.X == "" {
		t.Errorf("unexpected JWK: %+v", jwk)
	}
}
