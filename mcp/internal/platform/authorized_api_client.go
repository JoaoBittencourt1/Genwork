package platform

import (
	"context"
	"encoding/json"
	"net/http"
)

type AccessTokenSource interface {
	AccessToken(ctx context.Context) (string, error)
	Invalidate(rejectedAccessToken string)
}

// AuthorizedAPIClient sends every request as the signed-in user, so the API applies that user's own permissions.
type AuthorizedAPIClient struct {
	api    *APIClient
	tokens AccessTokenSource
}

func NewAuthorizedAPIClient(api *APIClient, tokens AccessTokenSource) *AuthorizedAPIClient {
	return &AuthorizedAPIClient{api: api, tokens: tokens}
}

// Send retries once with a fresh access token when the API rejects the current one before it expires (e.g. a key rotation).
func (client *AuthorizedAPIClient) Send(ctx context.Context, request APIRequest) (json.RawMessage, error) {
	accessToken, err := client.tokens.AccessToken(ctx)
	if err != nil {
		return nil, err
	}
	body, err := client.api.SendWithToken(ctx, request, accessToken)
	if !IsAPIStatus(err, http.StatusUnauthorized) {
		return body, err
	}
	client.tokens.Invalidate(accessToken)
	accessToken, err = client.tokens.AccessToken(ctx)
	if err != nil {
		return nil, err
	}
	return client.api.SendWithToken(ctx, request, accessToken)
}
