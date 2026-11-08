package mcpservers

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

const (
	// OAuthCallbackPath is the redirect URI every authorization server sends the browser back to.
	OAuthCallbackPath = "/api/genagents/v1/mcp-servers/oauth/callback"
	// OAuthResultPagePath is the web app page the callback lands on, with the outcome in the query.
	OAuthResultPagePath     = "/ai/mcp-servers"
	OAuthAuthorizationTTL   = 10 * time.Minute
	OAuthRefreshMargin      = 5 * time.Minute
	MaxOAuthClientIDLength  = 512
	MaxOAuthScopeLength     = 1024
	maxOAuthLastErrorLength = 300
)

var (
	ErrOAuthNotApplicable      = httpapi.NewValidationError(map[string]string{"transport": "oauth_requires_http"})
	ErrOAuthDiscoveryFailed    = httpapi.NewAPIError(http.StatusBadGateway, "oauth_discovery_failed")
	ErrOAuthRegistrationNeeded = httpapi.NewAPIError(http.StatusConflict, "oauth_client_id_required")
	ErrOAuthNotConnected       = httpapi.NewAPIError(http.StatusNotFound, "oauth_not_connected")
	ErrOAuthStateInvalid       = httpapi.NewAPIError(http.StatusBadRequest, "oauth_state_invalid")
)

type OAuthStatus string

const (
	OAuthPending   OAuthStatus = "pending"
	OAuthConnected OAuthStatus = "connected"
	OAuthError     OAuthStatus = "error"
)

// OAuthConnection is an MCP server's OAuth client registration and tokens; every secret in it is stored encrypted and
// none is ever part of a response (rule 67).
type OAuthConnection struct {
	ID                      int64
	ServerID                int64
	Status                  OAuthStatus
	Issuer                  *string
	AuthorizationEndpoint   *string
	TokenEndpoint           *string
	Resource                *string
	Scope                   *string
	ClientID                *string
	ClientSecretCiphertext  []byte
	TokenEndpointAuthMethod *string
	AccessTokenCiphertext   []byte
	RefreshTokenCiphertext  []byte
	ExpiresAt               *time.Time
	ConnectedAt             *time.Time
	LastError               *string
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

// NeedsRefresh says whether the access token expires soon enough that a run should not start with it.
func (connection OAuthConnection) NeedsRefresh(now time.Time) bool {
	return connection.ExpiresAt != nil && !connection.ExpiresAt.After(now.Add(OAuthRefreshMargin))
}

// OAuthAuthorization is one pending browser round trip: its state is stored hashed, its PKCE verifier encrypted.
type OAuthAuthorization struct {
	ID                     int64
	Token                  string
	ServerID               int64
	CodeVerifierCiphertext []byte
	ExpiresAt              time.Time
	ConsumedAt             *time.Time
}

// AuthorizationServerMetadata is the part of RFC 8414 metadata the flow uses.
type AuthorizationServerMetadata struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	RegistrationEndpoint              string   `json:"registration_endpoint"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
}

// OAuthDiscovery is what discovery learned about an MCP server's authorization.
type OAuthDiscovery struct {
	Resource        string
	ScopesSupported []string
	Metadata        AuthorizationServerMetadata
}

// OAuthClientRegistration is a client id, from dynamic registration or typed in by the admin.
type OAuthClientRegistration struct {
	ClientID                string
	ClientSecret            string
	TokenEndpointAuthMethod string
}

// OAuthTokenSet is a token endpoint answer; RefreshToken is empty when the server issued none.
type OAuthTokenSet struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
}

// IsSecureEndpoint accepts https, and plain http only on a loopback host.
func IsSecureEndpoint(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return false
	}
	if parsed.Scheme == "https" {
		return true
	}
	host := parsed.Hostname()
	if parsed.Scheme != "http" {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// CanonicalResource is the MCP server URI sent as the RFC 8707 resource: lowercase scheme and host, no fragment.
func CanonicalResource(serverURL string) string {
	parsed, err := url.Parse(serverURL)
	if err != nil {
		return serverURL
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""
	return parsed.String()
}

func truncateOAuthError(message string) string {
	if len(message) > maxOAuthLastErrorLength {
		return message[:maxOAuthLastErrorLength]
	}
	return message
}
