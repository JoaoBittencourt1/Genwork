package oauth

import (
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/auth"
)

// AuthorizationServerMetadataResponse is RFC 8414 metadata.
type AuthorizationServerMetadataResponse struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	RegistrationEndpoint              string   `json:"registration_endpoint"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	ScopesSupported                   []string `json:"scopes_supported"`
}

func newAuthorizationServerMetadataResponse(publicURL string) AuthorizationServerMetadataResponse {
	return AuthorizationServerMetadataResponse{
		Issuer:                            publicURL,
		AuthorizationEndpoint:             publicURL + "/oauth/authorize",
		TokenEndpoint:                     publicURL + "/api/core/v1/oauth/token",
		RegistrationEndpoint:              publicURL + "/api/core/v1/oauth/register",
		ResponseTypesSupported:            []string{"code"},
		GrantTypesSupported:               []string{grantAuthorizationCode, grantRefreshToken},
		CodeChallengeMethodsSupported:     []string{"S256"},
		TokenEndpointAuthMethodsSupported: []string{authMethodNone},
		ScopesSupported:                   []string{Scope},
	}
}

// ClientRegistrationResponse is the RFC 7591 answer; public clients never receive a secret.
type ClientRegistrationResponse struct {
	ClientID                string   `json:"client_id"`
	ClientIDIssuedAt        int64    `json:"client_id_issued_at"`
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
}

func newClientRegistrationResponse(client OAuthClient) ClientRegistrationResponse {
	return ClientRegistrationResponse{
		ClientID: client.Token, ClientIDIssuedAt: client.CreatedAt.Unix(), ClientName: client.Name, RedirectURIs: client.RedirectURIs,
		GrantTypes: []string{grantAuthorizationCode, grantRefreshToken}, ResponseTypes: []string{"code"}, TokenEndpointAuthMethod: authMethodNone,
	}
}

// AuthorizationPreviewResponse has either the consent screen's facts or the redirect that reports an error.
type AuthorizationPreviewResponse struct {
	ClientName  string `json:"clientName,omitempty"`
	Scope       string `json:"scope,omitempty"`
	RedirectURL string `json:"redirectUrl,omitempty"`
}

type AuthorizationRedirectResponse struct {
	RedirectURL string `json:"redirectUrl"`
}

// TokenResponse is RFC 6749 §5.1.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

func newTokenResponse(signIn auth.SignIn, now time.Time) TokenResponse {
	return TokenResponse{
		AccessToken: signIn.AccessToken, TokenType: "Bearer", ExpiresIn: int(signIn.AccessTokenExpiresAt.Sub(now).Round(time.Second).Seconds()),
		RefreshToken: signIn.RefreshToken, Scope: Scope,
	}
}

type protocolErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}
