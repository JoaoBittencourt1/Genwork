package oauth

import (
	"context"
	"errors"
	"net/url"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/auth"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/core/internal/sessions"
	"github.com/JoaoBittencourt1/Genwork/core/internal/users"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OAuthServiceSettings struct {
	// PublicURL is the issuer; the MCP server's resource is PublicURL + "/mcp" (RFC 8707).
	PublicURL string
}

type OAuthService struct {
	botLimiter *platform.RateLimiter
	botSecrets users.BotCredentialRepository
	db         *pgxpool.Pool
	auth       *auth.Service
	sessions   *sessions.Service
	clients    OAuthClientRepository
	codes      AuthorizationCodeRepository
	users      users.Repository
	settings   OAuthServiceSettings
	now        func() time.Time
}

// botTokenLimit is the rate limit of client-credentials token requests per client address and bot.
func NewOAuthService(db *pgxpool.Pool, authService *auth.Service, sessionService *sessions.Service, botTokenLimit platform.RateLimit, settings OAuthServiceSettings, now func() time.Time) *OAuthService {
	return &OAuthService{
		db: db, auth: authService, sessions: sessionService, settings: settings, now: now,
		botLimiter: platform.NewRateLimiter(botTokenLimit, now),
	}
}

func (service *OAuthService) MCPResource() string {
	return service.settings.PublicURL + "/mcp"
}

func (service *OAuthService) RegisterClient(ctx context.Context, request RegisterClientRequest) (OAuthClient, error) {
	return service.clients.Create(ctx, service.db, NewOAuthClient{Name: request.Name(), RedirectURIs: request.RedirectURIs, Now: service.now()})
}

// AuthorizationPreview is either what the consent screen shows, or the redirect that reports an error to the client.
type AuthorizationPreview struct {
	ClientName  string
	Scope       string
	RedirectURL string
}

func (service *OAuthService) PreviewAuthorization(ctx context.Context, request AuthorizationRequest) (AuthorizationPreview, error) {
	client, err := service.findClientFor(ctx, request)
	if err != nil {
		return AuthorizationPreview{}, err
	}
	if invalid := request.validateRedirectable(service.MCPResource()); invalid != nil {
		return AuthorizationPreview{RedirectURL: service.errorRedirect(request, invalid.code)}, nil
	}
	return AuthorizationPreview{ClientName: client.Name, Scope: Scope}, nil
}

// DecideAuthorization answers the URL to send the browser to: the code on approval, access_denied otherwise.
func (service *OAuthService) DecideAuthorization(ctx context.Context, userID int64, request AuthorizationRequest, decision Decision) (string, error) {
	client, err := service.findClientFor(ctx, request)
	if err != nil {
		return "", err
	}
	if invalid := request.validateRedirectable(service.MCPResource()); invalid != nil {
		return service.errorRedirect(request, invalid.code), nil
	}
	if decision == DecisionDeny {
		return service.errorRedirect(request, "access_denied"), nil
	}
	code, err := service.codes.Create(ctx, service.db, NewAuthorizationCode{
		ClientID: client.ID, UserID: userID, RedirectURI: request.RedirectURI, CodeChallenge: request.CodeChallenge, Scope: Scope, Now: service.now(),
	})
	if err != nil {
		return "", err
	}
	return redirectTo(request.RedirectURI, url.Values{"code": {code}, "state": {request.State}, "iss": {service.settings.PublicURL}}), nil
}

// findClientFor fails without a redirect when the redirect URI cannot be trusted (RFC 6749 §4.1.2.1).
func (service *OAuthService) findClientFor(ctx context.Context, request AuthorizationRequest) (OAuthClient, error) {
	client, err := service.clients.FindByToken(ctx, service.db, request.ClientID)
	if errors.Is(err, errClientNotFound) {
		return OAuthClient{}, ErrUnknownClient
	}
	if err != nil {
		return OAuthClient{}, err
	}
	if !client.HasRedirectURI(request.RedirectURI) {
		return OAuthClient{}, ErrUnregisteredRedirectURI
	}
	return client, nil
}

func (service *OAuthService) errorRedirect(request AuthorizationRequest, code string) string {
	return redirectTo(request.RedirectURI, url.Values{"error": {code}, "state": {request.State}, "iss": {service.settings.PublicURL}})
}

// ExchangeCode starts the assistant's session. A code presented twice means it leaked, so the session of its first
// exchange is revoked, as reused refresh tokens revoke theirs (rule 69).
func (service *OAuthService) ExchangeCode(ctx context.Context, request TokenRequest) (auth.SignIn, error) {
	client, err := service.findTokenClient(ctx, request.ClientID)
	if err != nil {
		return auth.SignIn{}, err
	}
	now := service.now()
	var signIn auth.SignIn
	var leakedSessionID *int64
	var outcome error
	err = pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		code, err := service.codes.LockByCode(ctx, tx, request.Code)
		if errors.Is(err, errCodeNotFound) {
			outcome = errInvalidGrant
			return nil
		}
		if err != nil {
			return err
		}
		if code.UsedAt != nil {
			leakedSessionID, outcome = code.SessionID, errInvalidGrant
			return nil
		}
		if code.IsExpiredAt(now) || code.ClientID != client.ID || code.RedirectURI != request.RedirectURI || !code.MatchesVerifier(request.CodeVerifier) {
			outcome = errInvalidGrant
			return nil
		}
		user, err := service.users.FindByID(ctx, tx, code.UserID)
		if errors.Is(err, users.ErrUserNotFound) || (err == nil && user.IsDeactivated()) {
			outcome = errInvalidGrant
			return nil
		}
		if err != nil {
			return err
		}
		if signIn, err = service.auth.StartOAuthSignIn(ctx, tx, user.ID, client.Name, client.ID); err != nil {
			return err
		}
		return service.codes.MarkUsed(ctx, tx, code.ID, signIn.Session.ID, now)
	})
	if err != nil {
		return auth.SignIn{}, err
	}
	if leakedSessionID != nil {
		if err := service.sessions.Revoke(ctx, *leakedSessionID); err != nil {
			return auth.SignIn{}, err
		}
	}
	if outcome != nil {
		return auth.SignIn{}, outcome
	}
	return signIn, nil
}

// Refresh rotates only a session this client obtained; every refusal is invalid_grant, so a client learns nothing more.
func (service *OAuthService) Refresh(ctx context.Context, request TokenRequest) (auth.SignIn, error) {
	client, err := service.findTokenClient(ctx, request.ClientID)
	if err != nil {
		return auth.SignIn{}, err
	}
	signIn, err := service.auth.RefreshForOAuthClient(ctx, request.RefreshToken, client.ID)
	var apiError *platform.APIError
	if errors.Is(err, sessions.ErrRefreshTokenOfOtherClient) || errors.As(err, &apiError) {
		return auth.SignIn{}, errInvalidGrant
	}
	return signIn, err
}

func (service *OAuthService) findTokenClient(ctx context.Context, clientID string) (OAuthClient, error) {
	client, err := service.clients.FindByToken(ctx, service.db, clientID)
	if errors.Is(err, errClientNotFound) {
		return OAuthClient{}, errUnknownClient
	}
	return client, err
}

// dummyBotSecretHash keeps unknown and deactivated bots as slow to refuse as a wrong secret (rule 68).
var dummyBotSecretHash = users.HashBotSecret("unknown bot")

// IssueBotToken answers the client_credentials grant. Unknown bot, deactivated bot, and wrong secret are the same
// refusal; the rate limit is the only distinguishable answer and applies per client address and bot token (rule 68).
// The bot token is public (members list), so keying on it alone would let anyone lock a bot out of its own runs.
func (service *OAuthService) IssueBotToken(ctx context.Context, clientAddress, botToken, secret string) (auth.SignIn, error) {
	if allowed, _ := service.botLimiter.Allow(clientAddress + "|" + botToken); !allowed {
		return auth.SignIn{}, platform.ErrRateLimited
	}
	credential, err := service.botSecrets.FindActiveByBotToken(ctx, service.db, botToken)
	if errors.Is(err, users.ErrBotCredentialNotFound) {
		credential = users.BotCredential{SecretHash: dummyBotSecretHash}
		credential.Matches(secret)
		return auth.SignIn{}, errInvalidBotClient
	}
	if err != nil {
		return auth.SignIn{}, err
	}
	if !credential.Matches(secret) {
		return auth.SignIn{}, errInvalidBotClient
	}
	bot, err := service.users.FindByID(ctx, service.db, credential.UserID)
	if err != nil {
		return auth.SignIn{}, err
	}
	var signIn auth.SignIn
	err = pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		signIn, err = service.auth.StartBotSignIn(ctx, tx, bot.ID, bot.Name)
		return err
	})
	return signIn, err
}
