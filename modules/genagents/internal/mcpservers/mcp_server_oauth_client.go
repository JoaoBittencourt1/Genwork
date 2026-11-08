package mcpservers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	oauthHTTPTimeout   = 10 * time.Second
	oauthMaxBodyBytes  = 1 << 20
	oauthClientName    = "Genwork"
	mcpProtocolVersion = "2025-06-18"
)

var resourceMetadataPattern = regexp.MustCompile(`resource_metadata="([^"]+)"`)

// McpOAuthHTTPClient speaks the MCP authorization flow over HTTP: protected resource metadata (RFC 9728), authorization
// server metadata (RFC 8414), dynamic client registration (RFC 7591), and the token endpoint. It knows nothing about
// storage (rule 9).
type McpOAuthHTTPClient struct {
	httpClient *http.Client
}

func NewMcpOAuthHTTPClient(httpClient *http.Client) *McpOAuthHTTPClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: oauthHTTPTimeout}
	}
	return &McpOAuthHTTPClient{httpClient: httpClient}
}

type protectedResourceMetadata struct {
	Resource             string   `json:"resource"`
	AuthorizationServers []string `json:"authorization_servers"`
	ScopesSupported      []string `json:"scopes_supported"`
}

// Discover finds the authorization server of an MCP server, falling back to the server's own origin as the 2025-03-26
// revision of the MCP spec did when a server publishes no resource metadata.
func (client *McpOAuthHTTPClient) Discover(ctx context.Context, serverURL string) (OAuthDiscovery, error) {
	discovery := OAuthDiscovery{Resource: CanonicalResource(serverURL)}
	resource, found := client.findResourceMetadata(ctx, serverURL)
	authorizationServer := originOf(serverURL)
	if found {
		if resource.Resource != "" {
			discovery.Resource = resource.Resource
		}
		discovery.ScopesSupported = resource.ScopesSupported
		if len(resource.AuthorizationServers) > 0 {
			authorizationServer = resource.AuthorizationServers[0]
		}
	}
	if !IsSecureEndpoint(authorizationServer) {
		return OAuthDiscovery{}, fmt.Errorf("%w: authorization server %q is not https", ErrOAuthDiscoveryFailed, authorizationServer)
	}
	metadata, err := client.findServerMetadata(ctx, authorizationServer)
	if err != nil {
		return OAuthDiscovery{}, err
	}
	discovery.Metadata = metadata
	return discovery, nil
}

func (client *McpOAuthHTTPClient) findResourceMetadata(ctx context.Context, serverURL string) (protectedResourceMetadata, bool) {
	candidates := []string{}
	if advertised := client.probeResourceMetadataURL(ctx, serverURL); advertised != "" {
		candidates = append(candidates, advertised)
	}
	candidates = append(candidates, wellKnownURLs(serverURL, "oauth-protected-resource")...)
	for _, candidate := range candidates {
		var metadata protectedResourceMetadata
		if IsSecureEndpoint(candidate) && client.getJSON(ctx, candidate, &metadata) == nil {
			return metadata, true
		}
	}
	return protectedResourceMetadata{}, false
}

// probeResourceMetadataURL calls the server without a token and reads the metadata URL its 401 advertises.
func (client *McpOAuthHTTPClient) probeResourceMetadataURL(ctx context.Context, serverURL string) string {
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"` + mcpProtocolVersion +
		`","capabilities":{},"clientInfo":{"name":"` + oauthClientName + `","version":"1"}}}`
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, serverURL, strings.NewReader(body))
	if err != nil {
		return ""
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	response, err := client.httpClient.Do(request)
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, oauthMaxBodyBytes))
	matches := resourceMetadataPattern.FindStringSubmatch(response.Header.Get("WWW-Authenticate"))
	if len(matches) != 2 {
		return ""
	}
	return matches[1]
}

func (client *McpOAuthHTTPClient) findServerMetadata(ctx context.Context, issuer string) (AuthorizationServerMetadata, error) {
	candidates := append(wellKnownURLs(issuer, "oauth-authorization-server"), wellKnownURLs(issuer, "openid-configuration")...)
	for _, candidate := range candidates {
		var metadata AuthorizationServerMetadata
		if client.getJSON(ctx, candidate, &metadata) == nil && metadata.AuthorizationEndpoint != "" && metadata.TokenEndpoint != "" {
			if !IsSecureEndpoint(metadata.AuthorizationEndpoint) || !IsSecureEndpoint(metadata.TokenEndpoint) {
				return AuthorizationServerMetadata{}, fmt.Errorf("%w: endpoints must be https", ErrOAuthDiscoveryFailed)
			}
			if metadata.Issuer == "" {
				metadata.Issuer = issuer
			}
			return metadata, nil
		}
	}
	return AuthorizationServerMetadata{}, fmt.Errorf("%w: no authorization server metadata at %s", ErrOAuthDiscoveryFailed, issuer)
}

// wellKnownURLs lists where RFC 8414 and RFC 9728 put metadata: inserted before the path, then at the root.
func wellKnownURLs(base, name string) []string {
	parsed, err := url.Parse(base)
	if err != nil || parsed.Host == "" {
		return nil
	}
	origin := parsed.Scheme + "://" + parsed.Host
	path := strings.TrimRight(parsed.Path, "/")
	urls := []string{}
	if path != "" {
		urls = append(urls, origin+"/.well-known/"+name+path)
		if name == "openid-configuration" {
			urls = append(urls, origin+path+"/.well-known/"+name)
		}
	}
	return append(urls, origin+"/.well-known/"+name)
}

func originOf(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return parsed.Scheme + "://" + parsed.Host
}

// Register creates a public client with dynamic registration.
func (client *McpOAuthHTTPClient) Register(ctx context.Context, registrationEndpoint, redirectURI string) (OAuthClientRegistration, error) {
	if registrationEndpoint == "" {
		return OAuthClientRegistration{}, ErrOAuthRegistrationNeeded
	}
	if !IsSecureEndpoint(registrationEndpoint) {
		return OAuthClientRegistration{}, fmt.Errorf("%w: registration endpoint is not https", ErrOAuthDiscoveryFailed)
	}
	body, err := json.Marshal(map[string]any{
		"client_name": oauthClientName, "redirect_uris": []string{redirectURI}, "grant_types": []string{"authorization_code", "refresh_token"},
		"response_types": []string{"code"}, "token_endpoint_auth_method": "none",
	})
	if err != nil {
		return OAuthClientRegistration{}, fmt.Errorf("encode registration: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, registrationEndpoint, bytes.NewReader(body))
	if err != nil {
		return OAuthClientRegistration{}, fmt.Errorf("build registration: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	var registered struct {
		ClientID                string `json:"client_id"`
		ClientSecret            string `json:"client_secret"`
		TokenEndpointAuthMethod string `json:"token_endpoint_auth_method"`
	}
	if err := client.doJSON(request, &registered); err != nil {
		return OAuthClientRegistration{}, fmt.Errorf("%w: registration: %v", ErrOAuthDiscoveryFailed, err)
	}
	if registered.ClientID == "" {
		return OAuthClientRegistration{}, fmt.Errorf("%w: registration returned no client id", ErrOAuthDiscoveryFailed)
	}
	return OAuthClientRegistration{ClientID: registered.ClientID, ClientSecret: registered.ClientSecret, TokenEndpointAuthMethod: registered.TokenEndpointAuthMethod}, nil
}

// TokenRequest is a call to the token endpoint; Grant holds the grant-specific fields.
type TokenRequest struct {
	Endpoint string
	Client   OAuthClientRegistration
	Resource string
	Grant    url.Values
}

// RequestToken calls the token endpoint, authenticating the client the way it registered: HTTP Basic by default for a
// confidential client (RFC 6749 §2.3.1), in the body for client_secret_post, and not at all for a public client.
func (client *McpOAuthHTTPClient) RequestToken(ctx context.Context, tokenRequest TokenRequest) (OAuthTokenSet, error) {
	form := url.Values{}
	for key, values := range tokenRequest.Grant {
		form[key] = values
	}
	if tokenRequest.Resource != "" {
		form.Set("resource", tokenRequest.Resource)
	}
	useBasic := tokenRequest.Client.ClientSecret != "" && tokenRequest.Client.TokenEndpointAuthMethod != "client_secret_post"
	if !useBasic {
		form.Set("client_id", tokenRequest.Client.ClientID)
		if tokenRequest.Client.ClientSecret != "" {
			form.Set("client_secret", tokenRequest.Client.ClientSecret)
		}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenRequest.Endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return OAuthTokenSet{}, fmt.Errorf("build token request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	if useBasic {
		request.SetBasicAuth(url.QueryEscape(tokenRequest.Client.ClientID), url.QueryEscape(tokenRequest.Client.ClientSecret))
	}
	var answer struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := client.doJSON(request, &answer); err != nil {
		return OAuthTokenSet{}, err
	}
	if answer.AccessToken == "" {
		return OAuthTokenSet{}, errors.New("the token endpoint returned no access token")
	}
	return OAuthTokenSet{AccessToken: answer.AccessToken, RefreshToken: answer.RefreshToken, ExpiresIn: answer.ExpiresIn}, nil
}

func (client *McpOAuthHTTPClient) getJSON(ctx context.Context, address string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("MCP-Protocol-Version", mcpProtocolVersion)
	return client.doJSON(request, target)
}

// doJSON never echoes a response body in its error: a token endpoint error body can repeat what was sent.
func (client *McpOAuthHTTPClient) doJSON(request *http.Request, target any) error {
	response, err := client.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("call %s: %w", request.URL.Host, err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, oauthMaxBodyBytes))
	if err != nil {
		return fmt.Errorf("read %s: %w", request.URL.Host, err)
	}
	if response.StatusCode >= http.StatusBadRequest {
		var failure struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(payload, &failure)
		if failure.Error != "" {
			return fmt.Errorf("%s answered %d: %s", request.URL.Host, response.StatusCode, failure.Error)
		}
		return fmt.Errorf("%s answered %d", request.URL.Host, response.StatusCode)
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("decode %s: %w", request.URL.Host, err)
	}
	return nil
}
