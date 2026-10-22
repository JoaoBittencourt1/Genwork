package githubsync

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const webhookSignaturePrefix = "sha256="

// VerifyWebhookSignature checks GitHub's X-Hub-Signature-256 header, the HMAC-SHA256 of the raw body, in constant time.
func VerifyWebhookSignature(secret GithubCredential, body []byte, signatureHeader string) bool {
	encoded, found := strings.CutPrefix(signatureHeader, webhookSignaturePrefix)
	if !found {
		return false
	}
	signature, err := hex.DecodeString(encoded)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, secret.Reveal())
	mac.Write(body)
	return hmac.Equal(signature, mac.Sum(nil))
}
