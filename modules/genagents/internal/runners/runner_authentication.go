package runners

import (
	"context"
	"net/http"
	"strings"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

// RunnerAuthenticator finds the runner a secret belongs to, refusing revoked runners.
type RunnerAuthenticator interface {
	Authenticate(ctx context.Context, secret string) (Runner, error)
}

type RunnerRequestHandler func(w http.ResponseWriter, r *http.Request, runner Runner)

// RequireRunner authenticates a runner route with the runner's own secret: those routes are public to the router, so
// this is their authorization rule (rule 18).
func RequireRunner(authenticator RunnerAuthenticator, next RunnerRequestHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		secret, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !found || strings.TrimSpace(secret) == "" {
			httpapi.WriteError(w, r, ErrInvalidRunnerSecret)
			return
		}
		runner, err := authenticator.Authenticate(r.Context(), strings.TrimSpace(secret))
		if err != nil {
			httpapi.WriteError(w, r, err)
			return
		}
		next(w, r, runner)
	}
}
