package auth

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

const KeyringService = "genwork-mcp"

// KeyringCredentialStore uses the OS secure store: Keychain, Windows Credential Manager, or Secret Service (rule 69).
type KeyringCredentialStore struct{}

func NewKeyringCredentialStore() *KeyringCredentialStore {
	return &KeyringCredentialStore{}
}

func (store *KeyringCredentialStore) Load(instanceURL string) (StoredCredential, error) {
	secret, err := keyring.Get(KeyringService, instanceURL)
	if errors.Is(err, keyring.ErrNotFound) {
		return StoredCredential{}, ErrNotSignedIn
	}
	if err != nil {
		return StoredCredential{}, fmt.Errorf("read credential from the OS secure store: %w", err)
	}
	var credential StoredCredential
	if err := json.Unmarshal([]byte(secret), &credential); err != nil || credential.RefreshToken == "" {
		return StoredCredential{}, ErrNotSignedIn
	}
	return credential, nil
}

func (store *KeyringCredentialStore) Save(instanceURL string, credential StoredCredential) error {
	secret, err := json.Marshal(credential)
	if err != nil {
		return fmt.Errorf("encode credential: %w", err)
	}
	if err := keyring.Set(KeyringService, instanceURL, string(secret)); err != nil {
		return fmt.Errorf("write credential to the OS secure store: %w", err)
	}
	return nil
}

func (store *KeyringCredentialStore) Delete(instanceURL string) error {
	err := keyring.Delete(KeyringService, instanceURL)
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("remove credential from the OS secure store: %w", err)
	}
	return nil
}
