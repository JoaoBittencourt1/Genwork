package remote

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/tokens"
	"github.com/modelcontextprotocol/go-sdk/auth"
)

// BearerTokenVerifier checks core-issued access tokens; the SDK's verifier implements it (rule 54).
type BearerTokenVerifier interface {
	Verify(ctx context.Context, rawToken string) (tokens.Principal, error)
}

// verifyBearerToken adapts the SDK verifier to go-sdk: UserID binds each MCP session to the user who opened it, so
// a request with another user's token on that session is refused.
func verifyBearerToken(verifier BearerTokenVerifier) auth.TokenVerifier {
	return func(ctx context.Context, rawToken string, _ *http.Request) (*auth.TokenInfo, error) {
		principal, err := verifier.Verify(ctx, rawToken)
		if errors.Is(err, tokens.ErrInvalidToken) {
			return nil, fmt.Errorf("%w: the access token is invalid or expired", auth.ErrInvalidToken)
		}
		if err != nil {
			return nil, err
		}
		return &auth.TokenInfo{
			UserID: principal.UserToken, Expiration: principal.ExpiresAt, Extra: map[string]any{accessTokenExtra: principal.AccessToken},
		}, nil
	}
}
