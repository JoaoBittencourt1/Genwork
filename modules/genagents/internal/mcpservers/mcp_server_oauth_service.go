package mcpservers

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	oauthClientSecretColumn = "mcp_server_oauth_connections.client_secret"
	oauthAccessTokenColumn  = "mcp_server_oauth_connections.access_token"
	oauthRefreshTokenColumn = "mcp_server_oauth_connections.refresh_token"
	oauthVerifierColumn     = "mcp_server_oauth_authorizations.code_verifier"
	authorizationHeader     = "Authorization"
)

// OAuthProviderClient is what the flow needs from the network (rule 9).
type OAuthProviderClient interface {
	Discover(ctx context.Context, serverURL string) (OAuthDiscovery, error)
	Register(ctx context.Context, registrationEndpoint, redirectURI string) (OAuthClientRegistration, error)
	RequestToken(ctx context.Context, request TokenRequest) (OAuthTokenSet, error)
}

// StartOAuthInput lets an admin give a client registered by hand, for authorization servers without registration.
type StartOAuthInput struct {
	ClientID     string
	ClientSecret string
	Scope        string
}

// OAuthCallbackOutcome is how the browser round trip ended, for the page the person lands on.
type OAuthCallbackOutcome struct {
	Connected bool
	Reason    string
}

// McpServerOAuthService connects HTTP MCP servers through OAuth: genagents is the OAuth client, keeps the tokens
// encrypted, refreshes them, and hands a bearer header to the runs that use the server.
type McpServerOAuthService struct {
	db        *pgxpool.Pool
	cipher    *platform.SecretCipher
	provider  OAuthProviderClient
	servers   McpServerRepository
	oauth     McpServerOAuthRepository
	publicURL string
	now       func() time.Time
}

func NewMcpServerOAuthService(db *pgxpool.Pool, cipher *platform.SecretCipher, provider OAuthProviderClient, publicURL string, now func() time.Time) *McpServerOAuthService {
	return &McpServerOAuthService{db: db, cipher: cipher, provider: provider, publicURL: strings.TrimRight(publicURL, "/"), now: now}
}

func (service *McpServerOAuthService) RedirectURI() string {
	return service.publicURL + OAuthCallbackPath
}

// ResultPageURL is where the callback sends the browser.
func (service *McpServerOAuthService) ResultPageURL(outcome OAuthCallbackOutcome) string {
	query := url.Values{"mcpOAuth": {"connected"}}
	if !outcome.Connected {
		query = url.Values{"mcpOAuth": {"failed"}, "reason": {outcome.Reason}}
	}
	return service.publicURL + OAuthResultPagePath + "?" + query.Encode()
}

// StartAuthorization discovers the server's authorization server, registers a client when none was given, and returns
// the URL the person's browser opens. The client is kept with the pending authorization: the active connection, if any,
// stays as it is until the new code exchange succeeds.
func (service *McpServerOAuthService) StartAuthorization(ctx context.Context, serverToken string, input StartOAuthInput) (string, error) {
	server, err := service.servers.FindByToken(ctx, service.db, serverToken)
	if err != nil {
		return "", err
	}
	if server.Transport != TransportHTTP {
		return "", ErrOAuthNotApplicable
	}
	discovery, err := service.provider.Discover(ctx, server.Config.URL)
	if err != nil {
		return "", wrapDiscoveryError(err)
	}
	client := OAuthClientRegistration{ClientID: input.ClientID, ClientSecret: input.ClientSecret}
	if client.ClientID == "" {
		if client, err = service.provider.Register(ctx, discovery.Metadata.RegistrationEndpoint, service.RedirectURI()); err != nil {
			return "", wrapDiscoveryError(err)
		}
	}
	scope := input.Scope
	if scope == "" {
		scope = strings.Join(discovery.ScopesSupported, " ")
	}
	verifier, state := platform.NewBearerSecret(), platform.NewBearerSecret()
	authorization := OAuthAuthorization{Token: platform.NewPublicToken(), ServerID: server.ID, ExpiresAt: service.now().Add(OAuthAuthorizationTTL)}
	if authorization.Setup, err = service.clientSetup(server, discovery, client); err != nil {
		return "", err
	}
	if authorization.CodeVerifierCiphertext, err = service.cipher.Encrypt(verifier, oauthVerifierColumn+"|"+authorization.Token); err != nil {
		return "", err
	}
	if err := service.oauth.CreateAuthorization(ctx, service.db, authorization, platform.HashBearerSecret(state), service.now()); err != nil {
		return "", err
	}
	return buildAuthorizationURL(discovery, client.ClientID, service.RedirectURI(), state, verifier, scope), nil
}

func wrapDiscoveryError(err error) error {
	if errors.Is(err, ErrOAuthRegistrationNeeded) {
		return ErrOAuthRegistrationNeeded
	}
	slog.Warn("mcp oauth discovery", "err", err)
	return ErrOAuthDiscoveryFailed
}

func (service *McpServerOAuthService) clientSetup(server StoredMcpServer, discovery OAuthDiscovery, client OAuthClientRegistration) (OAuthClientSetup, error) {
	setup := OAuthClientSetup{
		ServerURL: server.Config.URL, TokenEndpoint: discovery.Metadata.TokenEndpoint, Resource: discovery.Resource, ClientID: client.ClientID,
		TokenEndpointAuthMethod: client.TokenEndpointAuthMethod,
	}
	if client.ClientSecret != "" {
		ciphertext, err := service.cipher.Encrypt(client.ClientSecret, oauthClientSecretColumn+"|"+server.Token)
		if err != nil {
			return OAuthClientSetup{}, err
		}
		setup.ClientSecretCiphertext = ciphertext
	}
	return setup, nil
}

// buildAuthorizationURL adds PKCE (S256) and the RFC 8707 resource, which the MCP authorization spec requires.
func buildAuthorizationURL(discovery OAuthDiscovery, clientID, redirectURI, state, verifier, scope string) string {
	challenge := sha256.Sum256([]byte(verifier))
	query := url.Values{
		"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {redirectURI}, "state": {state},
		"code_challenge": {base64.RawURLEncoding.EncodeToString(challenge[:])}, "code_challenge_method": {"S256"}, "resource": {discovery.Resource},
	}
	if scope != "" {
		query.Set("scope", scope)
	}
	separator := "?"
	if strings.Contains(discovery.Metadata.AuthorizationEndpoint, "?") {
		separator = "&"
	}
	return discovery.Metadata.AuthorizationEndpoint + separator + query.Encode()
}

// CompleteAuthorization handles the browser coming back: it accepts a state once, then exchanges the code. The reason
// in a failed outcome is a fixed code, never text from the provider.
func (service *McpServerOAuthService) CompleteAuthorization(ctx context.Context, state, code, providerError string) OAuthCallbackOutcome {
	if state == "" {
		return OAuthCallbackOutcome{Reason: "invalid_state"}
	}
	authorization, err := service.oauth.ConsumeAuthorization(ctx, service.db, platform.HashBearerSecret(state), service.now())
	if err != nil {
		return OAuthCallbackOutcome{Reason: "invalid_state"}
	}
	if providerError != "" || code == "" {
		service.recordFailure(ctx, authorization.ServerID, "authorization_denied")
		return OAuthCallbackOutcome{Reason: "authorization_denied"}
	}
	if err := service.exchangeCode(ctx, authorization, code); err != nil {
		slog.Warn("mcp oauth token exchange", "server", authorization.ServerID, "err", err)
		service.recordFailure(ctx, authorization.ServerID, "token_exchange_failed")
		return OAuthCallbackOutcome{Reason: "token_exchange_failed"}
	}
	return OAuthCallbackOutcome{Connected: true}
}

var errServerURLChanged = errors.New("the server's URL changed during the authorization")

// exchangeCode trades the code with the authorization's own client and only then makes that client and its tokens the
// active connection, refusing when the server's URL changed meanwhile.
func (service *McpServerOAuthService) exchangeCode(ctx context.Context, authorization OAuthAuthorization, code string) error {
	server, err := service.servers.FindByID(ctx, service.db, authorization.ServerID)
	if err != nil {
		return err
	}
	if server.Config.URL != authorization.Setup.ServerURL {
		return errServerURLChanged
	}
	verifier, err := service.cipher.Decrypt(authorization.CodeVerifierCiphertext, oauthVerifierColumn+"|"+authorization.Token)
	if err != nil {
		return err
	}
	client, err := service.clientOf(server.Token, authorization.Setup)
	if err != nil {
		return err
	}
	tokens, err := service.provider.RequestToken(ctx, TokenRequest{
		Endpoint: authorization.Setup.TokenEndpoint, Client: client, Resource: authorization.Setup.Resource,
		Grant: url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {service.RedirectURI()}, "code_verifier": {verifier}},
	})
	if err != nil {
		return err
	}
	update, err := service.encryptTokens(server, tokens)
	if err != nil {
		return err
	}
	return service.oauth.Connect(ctx, service.db, server.ID, authorization.Setup, update)
}

func (service *McpServerOAuthService) clientOf(serverToken string, setup OAuthClientSetup) (OAuthClientRegistration, error) {
	client := OAuthClientRegistration{ClientID: setup.ClientID, TokenEndpointAuthMethod: setup.TokenEndpointAuthMethod}
	if len(setup.ClientSecretCiphertext) > 0 {
		secret, err := service.cipher.Decrypt(setup.ClientSecretCiphertext, oauthClientSecretColumn+"|"+serverToken)
		if err != nil {
			return OAuthClientRegistration{}, err
		}
		client.ClientSecret = secret
	}
	return client, nil
}

func (service *McpServerOAuthService) encryptTokens(server StoredMcpServer, tokens OAuthTokenSet) (OAuthTokenUpdate, error) {
	now := service.now()
	update := OAuthTokenUpdate{At: now}
	var err error
	if update.AccessTokenCiphertext, err = service.cipher.Encrypt(tokens.AccessToken, oauthAccessTokenColumn+"|"+server.Token); err != nil {
		return OAuthTokenUpdate{}, err
	}
	if tokens.RefreshToken != "" {
		if update.RefreshTokenCiphertext, err = service.cipher.Encrypt(tokens.RefreshToken, oauthRefreshTokenColumn+"|"+server.Token); err != nil {
			return OAuthTokenUpdate{}, err
		}
	}
	if tokens.ExpiresIn > 0 {
		expiresAt := now.Add(time.Duration(tokens.ExpiresIn) * time.Second)
		update.ExpiresAt = &expiresAt
	}
	return update, nil
}

func (service *McpServerOAuthService) recordFailure(ctx context.Context, serverID int64, message string) {
	if err := service.oauth.RecordFailedAttempt(context.WithoutCancel(ctx), service.db, serverID, message, service.now()); err != nil {
		slog.Warn("record mcp oauth failure", "server", serverID, "err", err)
	}
}

func (service *McpServerOAuthService) Disconnect(ctx context.Context, serverToken string) error {
	return pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		server, err := service.servers.FindByToken(ctx, tx, serverToken)
		if err != nil {
			return err
		}
		deleted, err := service.oauth.Delete(ctx, tx, server.ID)
		if err != nil {
			return err
		}
		if !deleted {
			return ErrOAuthNotConnected
		}
		return nil
	})
}

// ForgetConnection drops a server's connection and open authorizations when its URL changes: tokens issued for one
// server must never reach another, so the admin authorizes the new URL again.
func (service *McpServerOAuthService) ForgetConnection(ctx context.Context, db platform.DBTX, serverID int64) error {
	if _, err := service.oauth.Delete(ctx, db, serverID); err != nil {
		return err
	}
	return service.oauth.InvalidatePendingAuthorizations(ctx, db, serverID, service.now())
}

// AttachTokens adds the bearer header of every connected server, refreshing a token about to expire. A server whose
// refresh fails is marked in error and runs without the header, so one broken connection does not block the run.
func (service *McpServerOAuthService) AttachTokens(ctx context.Context, db platform.DBTX, servers []StoredMcpServer) error {
	ids := make([]int64, 0, len(servers))
	for _, server := range servers {
		ids = append(ids, server.ID)
	}
	connections, err := service.oauth.ListByServers(ctx, db, ids)
	if err != nil {
		return err
	}
	for index := range servers {
		connection, found := connections[servers[index].ID]
		if !found || connection.Status != OAuthConnected {
			continue
		}
		if !connection.IssuedFor(servers[index].Config.URL) {
			if err := service.oauth.MarkError(ctx, db, servers[index].ID, "server_url_changed", service.now()); err != nil {
				return err
			}
			continue
		}
		accessToken, err := service.currentAccessToken(ctx, db, servers[index], connection)
		if err != nil {
			slog.Warn("refresh mcp oauth token", "server", servers[index].Token, "err", err)
			if markErr := service.oauth.MarkError(ctx, db, servers[index].ID, "refresh_failed", service.now()); markErr != nil {
				return markErr
			}
			continue
		}
		withBearer(&servers[index], accessToken)
	}
	return nil
}

func (service *McpServerOAuthService) currentAccessToken(ctx context.Context, db platform.DBTX, server StoredMcpServer, connection OAuthConnection) (string, error) {
	if !connection.NeedsRefresh(service.now()) || len(connection.RefreshTokenCiphertext) == 0 {
		return service.cipher.Decrypt(connection.AccessTokenCiphertext, oauthAccessTokenColumn+"|"+server.Token)
	}
	refreshToken, err := service.cipher.Decrypt(connection.RefreshTokenCiphertext, oauthRefreshTokenColumn+"|"+server.Token)
	if err != nil {
		return "", err
	}
	client, err := service.clientOf(server.Token, connection.Setup)
	if err != nil {
		return "", err
	}
	tokens, err := service.provider.RequestToken(ctx, TokenRequest{
		Endpoint: connection.Setup.TokenEndpoint, Client: client, Resource: connection.Setup.Resource,
		Grant: url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refreshToken}},
	})
	if err != nil {
		return "", err
	}
	update, err := service.encryptTokens(server, tokens)
	if err != nil {
		return "", err
	}
	if err := service.oauth.RefreshTokens(ctx, db, server.ID, update); err != nil {
		return "", err
	}
	return tokens.AccessToken, nil
}

// withBearer replaces any Authorization header the admin typed with the OAuth one, as a secret entry.
func withBearer(server *StoredMcpServer, accessToken string) {
	headers := make([]Entry, 0, len(server.Config.Headers)+1)
	for _, header := range server.Config.Headers {
		if !strings.EqualFold(header.Name, authorizationHeader) {
			headers = append(headers, header)
		}
	}
	server.Config.Headers = append(headers, Entry{Name: authorizationHeader, Secret: true})
	if server.Secrets == nil {
		server.Secrets = map[string]string{}
	}
	server.Secrets[SecretKey("header", authorizationHeader)] = "Bearer " + accessToken
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
