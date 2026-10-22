// Package githubsynctest holds the fixtures and the fake GitHub server the githubsync tests use (rules 25, 26).
package githubsynctest

import (
	"bytes"
	"context"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/boardtest"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsync"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
)

var (
	EncryptionKey      = bytes.Repeat([]byte{0x42}, 32)
	OtherEncryptionKey = bytes.Repeat([]byte{0x17}, 32)
)

// The .invalid top-level domain never resolves, so a call that escapes the fakes fails instead of reaching GitHub (rule 25).
const (
	UnroutableGithubWebURL = "https://github.invalid"
	UnroutableGithubAPIURL = "https://api.github.invalid"
)

const (
	WebhookSecret = "fixture-webhook-secret"
	PrivateKey    = "-----BEGIN RSA PRIVATE KEY-----\nfixture-private-key\n-----END RSA PRIVATE KEY-----\n"
)

func NewCredentialCipher(t *testing.T, key []byte) *githubsync.CredentialCipher {
	t.Helper()
	cipher, err := githubsync.NewCredentialCipher(key)
	if err != nil {
		t.Fatalf("new credential cipher: %v", err)
	}
	return cipher
}

// StoreGithubApp saves a connected App whose credentials are encrypted with key.
func StoreGithubApp(t *testing.T, db platform.DBTX, key []byte) githubsync.GithubApp {
	t.Helper()
	cipher := NewCredentialCipher(t, key)
	token := platform.NewPublicToken()
	webhookSecret, err := cipher.Encrypt(githubsync.NewGithubCredential([]byte(WebhookSecret)), githubsync.WebhookSecretAssociatedData(token))
	if err != nil {
		t.Fatalf("encrypt webhook secret: %v", err)
	}
	privateKey, err := cipher.Encrypt(githubsync.NewGithubCredential([]byte(PrivateKey)), githubsync.PrivateKeyAssociatedData(token))
	if err != nil {
		t.Fatalf("encrypt private key: %v", err)
	}
	app, err := githubsync.GithubAppRepository{}.Create(context.Background(), db, githubsync.NewGithubApp{
		Token: token, GithubAppID: 4242, Slug: "genwork-fixture", OwnerLogin: "acme", AppURL: "https://github.test/apps/genwork-fixture",
		WebhookSecretCiphertext: webhookSecret, PrivateKeyCiphertext: privateKey, ConnectedBy: boardtest.CreatorToken, At: boardtest.FixedTime,
	})
	if err != nil {
		t.Fatalf("store github app: %v", err)
	}
	return app
}
