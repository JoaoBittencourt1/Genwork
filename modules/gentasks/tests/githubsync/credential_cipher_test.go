package githubsync_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsync"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsynctest"
)

func TestCredentialCipherRoundTrips(t *testing.T) {
	cipher := githubsynctest.NewCredentialCipher(t, githubsynctest.EncryptionKey)
	associatedData := githubsync.WebhookSecretAssociatedData("app-token")

	first, err := cipher.Encrypt(githubsync.NewGithubCredential([]byte("secret")), associatedData)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	second, _ := cipher.Encrypt(githubsync.NewGithubCredential([]byte("secret")), associatedData)
	decrypted, err := cipher.Decrypt(first, associatedData)

	if err != nil || string(decrypted.Reveal()) != "secret" {
		t.Fatalf("decrypted = %q, err = %v", decrypted.Reveal(), err)
	}
	if bytes.Contains(first, []byte("secret")) || bytes.Equal(first, second) {
		t.Error("ciphertexts must hide the plaintext and use a fresh nonce each time")
	}
}

func TestCredentialCipherRejectsTamperedCiphertext(t *testing.T) {
	cipher := githubsynctest.NewCredentialCipher(t, githubsynctest.EncryptionKey)
	associatedData := githubsync.PrivateKeyAssociatedData("app-token")
	ciphertext, _ := cipher.Encrypt(githubsync.NewGithubCredential([]byte("key")), associatedData)

	for _, tampered := range [][]byte{flipLastByte(ciphertext), ciphertext[:5], nil} {
		if _, err := cipher.Decrypt(tampered, associatedData); !errors.Is(err, githubsync.ErrCredentialUnreadable) {
			t.Errorf("tampered ciphertext error = %v", err)
		}
	}
}

func TestCredentialCipherBindsCiphertextToColumnAndRow(t *testing.T) {
	cipher := githubsynctest.NewCredentialCipher(t, githubsynctest.EncryptionKey)
	ciphertext, _ := cipher.Encrypt(githubsync.NewGithubCredential([]byte("secret")), githubsync.WebhookSecretAssociatedData("app-token"))

	for _, associatedData := range []string{githubsync.PrivateKeyAssociatedData("app-token"), githubsync.WebhookSecretAssociatedData("other-token")} {
		if _, err := cipher.Decrypt(ciphertext, associatedData); !errors.Is(err, githubsync.ErrCredentialUnreadable) {
			t.Errorf("moved ciphertext (%s) error = %v", associatedData, err)
		}
	}
}

func TestCredentialCipherFailsWithAnotherKey(t *testing.T) {
	associatedData := githubsync.WebhookSecretAssociatedData("app-token")
	ciphertext, _ := githubsynctest.NewCredentialCipher(t, githubsynctest.EncryptionKey).Encrypt(githubsync.NewGithubCredential([]byte("secret")), associatedData)

	_, err := githubsynctest.NewCredentialCipher(t, githubsynctest.OtherEncryptionKey).Decrypt(ciphertext, associatedData)

	if !errors.Is(err, githubsync.ErrCredentialUnreadable) {
		t.Errorf("error = %v", err)
	}
}

func TestNewCredentialCipherRejectsShortKeys(t *testing.T) {
	if _, err := githubsync.NewCredentialCipher(make([]byte, 31)); err == nil {
		t.Error("a key under 256 bits must be rejected")
	}
}

func flipLastByte(ciphertext []byte) []byte {
	tampered := bytes.Clone(ciphertext)
	tampered[len(tampered)-1] ^= 0xff
	return tampered
}
