package githubsync_test

import (
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsync"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsynctest"
)

func TestVerifyWebhookSignature(t *testing.T) {
	body := githubsynctest.WebhookPayload(t, githubsynctest.PullRequestOpened)
	secret := githubsync.NewGithubCredential([]byte(githubsynctest.WebhookSecret))
	valid := githubsynctest.SignWebhookPayload(githubsynctest.WebhookSecret, body)

	if !githubsync.VerifyWebhookSignature(secret, body, valid) {
		t.Error("a delivery signed with the secret must verify")
	}
	for _, signature := range []string{
		"",
		strings.TrimPrefix(valid, "sha256="),
		"sha1=" + strings.TrimPrefix(valid, "sha256="),
		"sha256=not-hex",
		"sha256=" + strings.Repeat("0", 64),
		githubsynctest.SignWebhookPayload("another-secret", body),
	} {
		if githubsync.VerifyWebhookSignature(secret, body, signature) {
			t.Errorf("signature %q must not verify", signature)
		}
	}
	if githubsync.VerifyWebhookSignature(secret, append(body, ' '), valid) {
		t.Error("a modified body must not verify")
	}
}
