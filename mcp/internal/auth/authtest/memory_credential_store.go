// Package authtest holds in-memory stand-ins for the OS secure store and for core's session endpoints.
package authtest

import (
	"sync"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/auth"
)

type MemoryCredentialStore struct {
	mutex       sync.Mutex
	credentials map[string]auth.StoredCredential
}

func NewMemoryCredentialStore() *MemoryCredentialStore {
	return &MemoryCredentialStore{credentials: map[string]auth.StoredCredential{}}
}

func (store *MemoryCredentialStore) Load(instanceURL string) (auth.StoredCredential, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	credential, ok := store.credentials[instanceURL]
	if !ok {
		return auth.StoredCredential{}, auth.ErrNotSignedIn
	}
	return credential, nil
}

func (store *MemoryCredentialStore) Save(instanceURL string, credential auth.StoredCredential) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	store.credentials[instanceURL] = credential
	return nil
}

func (store *MemoryCredentialStore) Delete(instanceURL string) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	delete(store.credentials, instanceURL)
	return nil
}
