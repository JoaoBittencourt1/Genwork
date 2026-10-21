package auth

import (
	"net/http"
	"strings"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/core/internal/sessions"
)

type Authenticator struct {
	issuer   *AccessTokenIssuer
	sessions *sessions.Service
}

func NewAuthenticator(issuer *AccessTokenIssuer, sessionService *sessions.Service) *Authenticator {
	return &Authenticator{issuer: issuer, sessions: sessionService}
}

// RequireAuthentication also checks that the session is still active, so revoking a session takes effect at once in core.
func (authenticator *Authenticator) RequireAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawToken, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !found {
			platform.WriteError(w, r, platform.ErrInvalidToken)
			return
		}
		claims, err := authenticator.issuer.Verify(strings.TrimSpace(rawToken))
		if err != nil {
			platform.WriteError(w, r, platform.ErrInvalidToken)
			return
		}
		session, err := authenticator.sessions.FindActive(r.Context(), claims.SessionToken)
		if err != nil || session.UserToken != claims.Subject {
			platform.WriteError(w, r, platform.ErrInvalidToken)
			return
		}
		ctx := platform.WithPrincipal(r.Context(), platform.Principal{
			UserID:       session.UserID,
			UserToken:    session.UserToken,
			RoleID:       session.RoleID,
			SessionID:    session.ID,
			SessionToken: session.Token,
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
