package auth

import (
	"context"
	"errors"
)

var (
	ErrNotSignedIn        = errors.New("not signed in to this Genwork instance; run `genwork-mcp login` first")
	ErrSessionEnded       = errors.New("the Genwork session expired or was revoked; run `genwork-mcp login` again")
	ErrMissingCredentials = errors.New("e-mail and password are required")
)

// StoredCredential is what survives between runs; the access token lives only in memory (rule 69).
type StoredCredential struct {
	Email        string `json:"email"`
	Session      string `json:"session"`
	RefreshToken string `json:"refreshToken"`
}

// CredentialStore keeps one credential per instance URL; Load answers ErrNotSignedIn when there is none.
type CredentialStore interface {
	Load(instanceURL string) (StoredCredential, error)
	Save(instanceURL string, credential StoredCredential) error
	Delete(instanceURL string) error
}

// RefreshLock serializes refreshes across every genwork-mcp process of the user, because refresh tokens are single-use.
type RefreshLock interface {
	Acquire(ctx context.Context) (release func(), err error)
}
