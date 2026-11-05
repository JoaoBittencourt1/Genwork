package platform_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform"
)

var testKey = bytes.Repeat([]byte{7}, 32)

func TestSecretCipherRoundTrip(t *testing.T) {
	secretCipher, err := platform.NewSecretCipher(testKey)
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}

	ciphertext, err := secretCipher.Encrypt("sk-live-secret", "ai_accounts.api_key|token-1")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	plaintext, err := secretCipher.Decrypt(ciphertext, "ai_accounts.api_key|token-1")

	if err != nil || plaintext != "sk-live-secret" || strings.Contains(string(ciphertext), "sk-live-secret") {
		t.Errorf("plaintext = %q, err = %v", plaintext, err)
	}
}

func TestSecretCipherFailsOnAssociatedDataMismatch(t *testing.T) {
	secretCipher, _ := platform.NewSecretCipher(testKey)
	ciphertext, _ := secretCipher.Encrypt("secret", "ai_accounts.api_key|token-1")

	if _, err := secretCipher.Decrypt(ciphertext, "ai_accounts.api_key|token-2"); err == nil {
		t.Error("decrypting for another row succeeded")
	}
	if _, err := secretCipher.Decrypt(ciphertext[:5], "ai_accounts.api_key|token-1"); err == nil {
		t.Error("decrypting a truncated ciphertext succeeded")
	}
}

func TestSecretCipherUsesAFreshNonce(t *testing.T) {
	secretCipher, _ := platform.NewSecretCipher(testKey)

	first, _ := secretCipher.Encrypt("secret", "a")
	second, _ := secretCipher.Encrypt("secret", "a")

	if bytes.Equal(first, second) {
		t.Error("two encryptions of the same secret are identical")
	}
}

func TestSecretCipherRejectsShortKeys(t *testing.T) {
	if _, err := platform.NewSecretCipher(testKey[:31]); err == nil {
		t.Error("a 31-byte key was accepted")
	}
}
