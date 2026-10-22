package remote

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/auth"
)

const accessTokenExtra = "accessToken"

var errNoRequestAccessToken = errors.New("the MCP request carried no access token")

type accessTokenKey struct{}

func withAccessToken(ctx context.Context, accessToken string) context.Context {
	return context.WithValue(ctx, accessTokenKey{}, accessToken)
}

// accessTokenOf reads the bearer token the verifier kept for the request; the server never stores it beyond (spec
// "Calls act with the request's token").
func accessTokenOf(info *auth.TokenInfo) string {
	if info == nil {
		return ""
	}
	token, _ := info.Extra[accessTokenExtra].(string)
	return token
}

// requestAccessTokenSource answers the token of the MCP request being handled, so every tool call acts as the user
// who sent it, with whatever token their client refreshed to.
type requestAccessTokenSource struct{}

func (requestAccessTokenSource) AccessToken(ctx context.Context) (string, error) {
	token, _ := ctx.Value(accessTokenKey{}).(string)
	if token == "" {
		return "", errNoRequestAccessToken
	}
	return token, nil
}

// Invalidate has nothing to drop: the client refreshes its own token and sends it with the next request.
func (requestAccessTokenSource) Invalidate(string) {}
