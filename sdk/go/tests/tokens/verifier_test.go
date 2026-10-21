package tokens_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/coretest"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/tokens"
)

type manualClock struct {
	mutex   sync.Mutex
	current time.Time
}

func (clock *manualClock) Now() time.Time {
	clock.mutex.Lock()
	defer clock.mutex.Unlock()
	return clock.current
}

func (clock *manualClock) Advance(duration time.Duration) {
	clock.mutex.Lock()
	defer clock.mutex.Unlock()
	clock.current = clock.current.Add(duration)
}

func newVerifier(core *coretest.FakeCore, clock *manualClock) *tokens.TokenVerifier {
	return tokens.NewTokenVerifier(tokens.TokenVerifierConfig{JWKSURL: core.JWKSURL(), Issuer: coretest.Issuer, Now: clock.Now})
}

func TestVerifyAcceptsCoreToken(t *testing.T) {
	core := coretest.NewFakeCore(t)
	verifier := newVerifier(core, &manualClock{current: time.Now()})
	token := core.MintAccessToken(t, "user-token", "gentasks.board.read")

	principal, err := verifier.Verify(context.Background(), token)

	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if principal.UserToken != "user-token" || principal.AccessToken != token || !principal.HasPermission("gentasks.board.read") || principal.HasPermission("gentasks.task.delete") {
		t.Errorf("principal = %+v", principal)
	}
}

func TestVerifyCachesKeys(t *testing.T) {
	core := coretest.NewFakeCore(t)
	verifier := newVerifier(core, &manualClock{current: time.Now()})

	for range 3 {
		if _, err := verifier.Verify(context.Background(), core.MintAccessToken(t, "user-token")); err != nil {
			t.Fatalf("verify: %v", err)
		}
	}

	if core.JWKSRequests() != 1 {
		t.Errorf("jwks requests = %d, want 1", core.JWKSRequests())
	}
}

func TestVerifyRejectsInvalidTokens(t *testing.T) {
	core := coretest.NewFakeCore(t)
	otherCore := coretest.NewFakeCore(t)
	clock := &manualClock{current: time.Now()}
	verifier := newVerifier(core, clock)
	wrongIssuer := tokens.NewTokenVerifier(tokens.TokenVerifierConfig{JWKSURL: core.JWKSURL(), Issuer: "http://elsewhere", Now: clock.Now})
	valid := core.MintAccessToken(t, "user-token")

	cases := map[string]func() error{
		"garbage": func() error { _, err := verifier.Verify(context.Background(), "abc.def.ghi"); return err },
		"expired": func() error {
			_, err := verifier.Verify(context.Background(), core.MintAccessTokenExpiringAt(t, "user-token", time.Now().Add(-time.Minute)))
			return err
		},
		"other key": func() error {
			_, err := verifier.Verify(context.Background(), otherCore.MintAccessToken(t, "user-token"))
			return err
		},
		"wrong issuer": func() error { _, err := wrongIssuer.Verify(context.Background(), valid); return err },
		"missing subject": func() error {
			_, err := verifier.Verify(context.Background(), core.MintAccessToken(t, ""))
			return err
		},
	}
	for name, verify := range cases {
		t.Run(name, func(t *testing.T) {
			if err := verify(); !errors.Is(err, tokens.ErrInvalidToken) {
				t.Errorf("error = %v, want ErrInvalidToken", err)
			}
		})
	}
}

func TestVerifyRefetchesKeysForNewKeyIDAtMostOncePerInterval(t *testing.T) {
	core := coretest.NewFakeCore(t)
	clock := &manualClock{current: time.Now()}
	verifier := newVerifier(core, clock)
	if _, err := verifier.Verify(context.Background(), core.MintAccessToken(t, "user-token")); err != nil {
		t.Fatalf("verify: %v", err)
	}

	core.RotateKey(t)
	rotated := core.MintAccessToken(t, "user-token")

	if _, err := verifier.Verify(context.Background(), rotated); !errors.Is(err, tokens.ErrInvalidToken) {
		t.Fatalf("within the interval the unknown key must be rejected, got %v", err)
	}
	clock.Advance(tokens.DefaultMinRefetchInterval)
	if _, err := verifier.Verify(context.Background(), rotated); err != nil {
		t.Fatalf("after the interval the new key must be fetched: %v", err)
	}
	if core.JWKSRequests() != 2 {
		t.Errorf("jwks requests = %d, want 2", core.JWKSRequests())
	}
}

func TestVerifyReportsUnavailableKeys(t *testing.T) {
	core := coretest.NewFakeCore(t)
	token := core.MintAccessToken(t, "user-token")
	verifier := tokens.NewTokenVerifier(tokens.TokenVerifierConfig{JWKSURL: "http://127.0.0.1:1/jwks", Issuer: coretest.Issuer})

	if _, err := verifier.Verify(context.Background(), token); !errors.Is(err, tokens.ErrKeysUnavailable) {
		t.Errorf("error = %v, want ErrKeysUnavailable", err)
	}
}

func TestVerifyReportsUnavailableKeysOnCoreError(t *testing.T) {
	core := coretest.NewFakeCore(t)
	token := core.MintAccessToken(t, "user-token")
	verifier := tokens.NewTokenVerifier(tokens.TokenVerifierConfig{JWKSURL: core.URL() + "/missing", Issuer: coretest.Issuer})

	if _, err := verifier.Verify(context.Background(), token); !errors.Is(err, tokens.ErrKeysUnavailable) {
		t.Errorf("error = %v, want ErrKeysUnavailable", err)
	}
}
