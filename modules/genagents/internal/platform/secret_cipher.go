package platform

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
)

var ErrCipherKeyTooShort = errors.New("encryption key is shorter than 32 bytes")

// SecretCipher encrypts secrets genagents must use again, such as provider API keys and bot client secrets, with
// AES-256-GCM. The associated data names the column and the row token, so a ciphertext copied to another row or
// column fails to decrypt (rule 67).
type SecretCipher struct {
	aead cipher.AEAD
}

func NewSecretCipher(key []byte) (*SecretCipher, error) {
	if len(key) < MinEncryptionKeyBytes {
		return nil, ErrCipherKeyTooShort
	}
	block, err := aes.NewCipher(key[:MinEncryptionKeyBytes])
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	return &SecretCipher{aead: aead}, nil
}

func (secretCipher *SecretCipher) Encrypt(plaintext, associatedData string) ([]byte, error) {
	nonce := make([]byte, secretCipher.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("read nonce: %w", err)
	}
	return secretCipher.aead.Seal(nonce, nonce, []byte(plaintext), []byte(associatedData)), nil
}

func (secretCipher *SecretCipher) Decrypt(ciphertext []byte, associatedData string) (string, error) {
	nonceSize := secretCipher.aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext is too short")
	}
	plaintext, err := secretCipher.aead.Open(nil, ciphertext[:nonceSize], ciphertext[nonceSize:], []byte(associatedData))
	if err != nil {
		return "", errors.New("decrypt secret: authentication failed")
	}
	return string(plaintext), nil
}
