package oauth

import (
	"net"
	"net/url"
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	defaultClientName      = "MCP client"
	maxClientNameLength    = 100
	maxRedirectURIs        = 5
	maxRedirectURILength   = 2000
	authMethodNone         = "none"
	grantAuthorizationCode = "authorization_code"
	grantRefreshToken      = "refresh_token"
)

// RegisterClientRequest is RFC 7591 client metadata. Unknown metadata is ignored, as the RFC requires, so the
// handler decodes it leniently.
type RegisterClientRequest struct {
	ClientName              *string  `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	TokenEndpointAuthMethod *string  `json:"token_endpoint_auth_method"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
}

func (request RegisterClientRequest) Validate() error {
	if len(request.RedirectURIs) == 0 || len(request.RedirectURIs) > maxRedirectURIs {
		return newProtocolError("invalid_redirect_uri", "register 1 to 5 redirect URIs")
	}
	for _, uri := range request.RedirectURIs {
		if !isAllowedRedirectURI(uri) {
			return newProtocolError("invalid_redirect_uri", "redirect URIs use https, or http on a loopback host, without a fragment")
		}
	}
	if request.ClientName != nil {
		if length := utf8.RuneCountInString(strings.TrimSpace(*request.ClientName)); length == 0 || length > maxClientNameLength {
			return newProtocolError("invalid_client_metadata", "client_name has 1 to 100 characters")
		}
	}
	if request.TokenEndpointAuthMethod != nil && *request.TokenEndpointAuthMethod != authMethodNone {
		return newProtocolError("invalid_client_metadata", "only public clients (token_endpoint_auth_method none) can register")
	}
	for _, grant := range request.GrantTypes {
		if grant != grantAuthorizationCode && grant != grantRefreshToken {
			return newProtocolError("invalid_client_metadata", "only the authorization_code and refresh_token grants are supported")
		}
	}
	for _, responseType := range request.ResponseTypes {
		if responseType != "code" {
			return newProtocolError("invalid_client_metadata", "only the code response type is supported")
		}
	}
	return nil
}

func (request RegisterClientRequest) Name() string {
	if request.ClientName == nil {
		return defaultClientName
	}
	return strings.TrimSpace(*request.ClientName)
}

// isAllowedRedirectURI accepts https anywhere, and http only on a loopback host, where native clients listen.
func isAllowedRedirectURI(raw string) bool {
	if len(raw) > maxRedirectURILength {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.Fragment != "" || strings.Contains(raw, "#") || parsed.User != nil {
		return false
	}
	switch parsed.Scheme {
	case "https":
		return true
	case "http":
		host := parsed.Hostname()
		return host == "localhost" || slices.Contains([]string{"127.0.0.1", "::1"}, net.ParseIP(host).String())
	default:
		return false
	}
}

type Decision string

const (
	DecisionAllow Decision = "allow"
	DecisionDeny  Decision = "deny"
)

// AuthorizationDecisionRequest is what the consent page posts: the request it showed, and the user's choice.
type AuthorizationDecisionRequest struct {
	ResponseType        string   `json:"responseType"`
	ClientID            string   `json:"clientId"`
	RedirectURI         string   `json:"redirectUri"`
	State               string   `json:"state"`
	CodeChallenge       string   `json:"codeChallenge"`
	CodeChallengeMethod string   `json:"codeChallengeMethod"`
	Scope               string   `json:"scope"`
	Resource            string   `json:"resource"`
	Decision            Decision `json:"decision"`
}

func (request AuthorizationDecisionRequest) Validate() map[string]string {
	if request.Decision != DecisionAllow && request.Decision != DecisionDeny {
		return map[string]string{"decision": "invalid"}
	}
	return nil
}

func (request AuthorizationDecisionRequest) AuthorizationRequest() AuthorizationRequest {
	return AuthorizationRequest{
		ResponseType: request.ResponseType, ClientID: request.ClientID, RedirectURI: request.RedirectURI, State: request.State,
		CodeChallenge: request.CodeChallenge, CodeChallengeMethod: request.CodeChallengeMethod, Scope: request.Scope, Resource: request.Resource,
	}
}

// AuthorizationRequestFromQuery reads the consent page's query, the parameters /oauth/authorize received.
func AuthorizationRequestFromQuery(query url.Values) AuthorizationRequest {
	return AuthorizationRequest{
		ResponseType: query.Get("response_type"), ClientID: query.Get("client_id"), RedirectURI: query.Get("redirect_uri"),
		State: query.Get("state"), CodeChallenge: query.Get("code_challenge"), CodeChallengeMethod: query.Get("code_challenge_method"),
		Scope: query.Get("scope"), Resource: query.Get("resource"),
	}
}

// TokenRequest is the form of POST /oauth/token for both grants.
type TokenRequest struct {
	GrantType    string
	Code         string
	RedirectURI  string
	ClientID     string
	CodeVerifier string
	RefreshToken string
}

// tokenParameters may each appear once (RFC 6749 §3.2).
var tokenParameters = []string{"grant_type", "code", "redirect_uri", "client_id", "code_verifier", "refresh_token"}

func TokenRequestFromForm(form url.Values) (TokenRequest, error) {
	for _, name := range tokenParameters {
		if len(form[name]) > 1 {
			return TokenRequest{}, newProtocolError("invalid_request", name+" appears more than once")
		}
	}
	request := TokenRequest{
		GrantType: form.Get("grant_type"), Code: form.Get("code"), RedirectURI: form.Get("redirect_uri"), ClientID: form.Get("client_id"),
		CodeVerifier: form.Get("code_verifier"), RefreshToken: form.Get("refresh_token"),
	}
	return request, request.validate()
}

func (request TokenRequest) validate() error {
	switch request.GrantType {
	case "":
		return newProtocolError("invalid_request", "grant_type is required")
	case grantAuthorizationCode:
		if request.Code == "" || request.RedirectURI == "" || request.ClientID == "" || !pkceValuePattern.MatchString(request.CodeVerifier) {
			return newProtocolError("invalid_request", "code, redirect_uri, client_id, and a code_verifier of 43 to 128 characters are required")
		}
	case grantRefreshToken:
		if request.RefreshToken == "" || request.ClientID == "" {
			return newProtocolError("invalid_request", "refresh_token and client_id are required")
		}
	default:
		return errUnsupportedGrantType
	}
	return nil
}
