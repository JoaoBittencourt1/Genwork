package oauth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"time"
)

const AuthorizationCodeLifetime = 5 * time.Minute

// AuthorizationCode is bound to the client, redirect URI, user, and PKCE challenge it was issued for; core keeps only
// its hash (rule 67).
type AuthorizationCode struct {
	ID            int64
	ClientID      int64
	UserID        int64
	RedirectURI   string
	CodeChallenge string
	Scope         string
	ExpiresAt     time.Time
	UsedAt        *time.Time
	// SessionID is the session the code was exchanged for, revoked if the code is presented again.
	SessionID *int64
}

func (code AuthorizationCode) IsExpiredAt(now time.Time) bool {
	return !now.Before(code.ExpiresAt)
}

// MatchesVerifier applies S256, the only method core accepts (RFC 7636 §4.6).
func (code AuthorizationCode) MatchesVerifier(verifier string) bool {
	digest := sha256.Sum256([]byte(verifier))
	return subtle.ConstantTimeCompare([]byte(base64.RawURLEncoding.EncodeToString(digest[:])), []byte(code.CodeChallenge)) == 1
}
