package modules

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
)

// CredentialAuthenticator keeps only a digest of the module secret so the clear value is not held after startup (rule 67).
type CredentialAuthenticator struct {
	secretDigest [sha256.Size]byte
}

func NewCredentialAuthenticator(secret string) *CredentialAuthenticator {
	return &CredentialAuthenticator{secretDigest: sha256.Sum256([]byte(secret))}
}

func (authenticator *CredentialAuthenticator) RequireModuleCredential(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		presented, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		presentedDigest := sha256.Sum256([]byte(strings.TrimSpace(presented)))
		if !found || subtle.ConstantTimeCompare(presentedDigest[:], authenticator.secretDigest[:]) != 1 {
			platform.WriteError(w, r, ErrInvalidModuleSecret)
			return
		}
		next.ServeHTTP(w, r)
	})
}
