package githubsync

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
)

// Each purpose derives its own key from GENTASKS_ENCRYPTION_KEY, so a key never serves two uses (design §3, §4).
const (
	credentialKeyInfo    = "gentasks/github-credentials/v1"
	manifestStateKeyInfo = "gentasks/github-manifest-state/v1"
	derivedKeyBytes      = 32
)

var ErrCredentialUnreadable = errors.New("github credential cannot be decrypted with the current encryption key")

// CredentialCipher encrypts the GitHub App's credentials with AES-256-GCM, stored as nonce followed by ciphertext.
type CredentialCipher struct {
	aead cipher.AEAD
}

func NewCredentialCipher(encryptionKey []byte) (*CredentialCipher, error) {
	key, err := deriveKey(encryptionKey, credentialKeyInfo)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	return &CredentialCipher{aead: aead}, nil
}

// WebhookSecretAssociatedData binds a ciphertext to its column and row, so it cannot be swapped into another one.
func WebhookSecretAssociatedData(appToken string) string {
	return "github_apps.webhook_secret_ciphertext:" + appToken
}

func PrivateKeyAssociatedData(appToken string) string {
	return "github_apps.private_key_ciphertext:" + appToken
}

func (credentialCipher *CredentialCipher) Encrypt(credential GithubCredential, associatedData string) ([]byte, error) {
	nonce := make([]byte, credentialCipher.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("read nonce: %w", err)
	}
	return credentialCipher.aead.Seal(nonce, nonce, credential.Reveal(), []byte(associatedData)), nil
}

func (credentialCipher *CredentialCipher) Decrypt(ciphertext []byte, associatedData string) (GithubCredential, error) {
	nonceSize := credentialCipher.aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return GithubCredential{}, ErrCredentialUnreadable
	}
	plaintext, err := credentialCipher.aead.Open(nil, ciphertext[:nonceSize], ciphertext[nonceSize:], []byte(associatedData))
	if err != nil {
		return GithubCredential{}, ErrCredentialUnreadable
	}
	return NewGithubCredential(plaintext), nil
}

func deriveKey(encryptionKey []byte, info string) ([]byte, error) {
	if len(encryptionKey) < platform.MinEncryptionKeyBytes {
		return nil, fmt.Errorf("encryption key must have at least %d bytes", platform.MinEncryptionKeyBytes)
	}
	key, err := hkdf.Key(sha256.New, encryptionKey, nil, info, derivedKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}
	return key, nil
}
