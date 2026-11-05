// Package corelink is genagents' HTTP client for core: bot members, bot tokens, and delegated sessions (rule 54).
package corelink

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

const DefaultTimeout = 5 * time.Second

var ErrCoreUnavailable = httpapi.NewAPIError(http.StatusServiceUnavailable, "core_unavailable")

// RefusalError is core refusing an operation; Status and Code are core's, relayed so callers can map them.
type RefusalError struct {
	Status int
	Code   string
	Fields map[string]string
}

func (err *RefusalError) Error() string {
	return fmt.Sprintf("core refused the request: %d %s", err.Status, err.Code)
}

// ToAPIError relays a core refusal as the same status and code, so a forbidden or invalid request keeps its meaning.
func (err *RefusalError) ToAPIError() *httpapi.APIError {
	return &httpapi.APIError{Status: err.Status, Code: err.Code, Fields: err.Fields}
}

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(coreURL string, timeout time.Duration) *Client {
	return &Client{baseURL: strings.TrimRight(coreURL, "/"), httpClient: &http.Client{Timeout: timeout}}
}

type BotMember struct {
	Token        string `json:"token"`
	ClientSecret string `json:"clientSecret"`
}

type SignIn struct {
	AccessToken          string    `json:"accessToken"`
	AccessTokenExpiresAt time.Time `json:"accessTokenExpiresAt"`
	RefreshToken         string    `json:"refreshToken"`
	Session              string    `json:"session"`
}

type BotAccessToken struct {
	AccessToken string
	ExpiresAt   time.Time
}

func (client *Client) CreateBotMember(ctx context.Context, accessToken, name, roleToken string) (BotMember, error) {
	var member BotMember
	err := client.callJSON(ctx, http.MethodPost, "/api/core/v1/bots", accessToken, map[string]any{"name": name, "role": roleToken}, &member)
	return member, err
}

func (client *Client) RenameBotMember(ctx context.Context, accessToken, botToken, name string) error {
	return client.patchBot(ctx, accessToken, botToken, map[string]any{"name": name})
}

func (client *Client) ChangeBotMemberRole(ctx context.Context, accessToken, botToken, roleToken string) error {
	return client.patchBot(ctx, accessToken, botToken, map[string]any{"role": roleToken})
}

func (client *Client) SetBotMemberActive(ctx context.Context, accessToken, botToken string, active bool) error {
	status := "deactivated"
	if active {
		status = "active"
	}
	return client.patchBot(ctx, accessToken, botToken, map[string]any{"status": status})
}

func (client *Client) patchBot(ctx context.Context, accessToken, botToken string, body map[string]any) error {
	return client.callJSON(ctx, http.MethodPatch, "/api/core/v1/bots/"+url.PathEscape(botToken), accessToken, body, nil)
}

func (client *Client) RemoveBotMember(ctx context.Context, accessToken, botToken string) error {
	return client.callJSON(ctx, http.MethodDelete, "/api/core/v1/bots/"+url.PathEscape(botToken), accessToken, nil, nil)
}

func (client *Client) RotateBotMemberSecret(ctx context.Context, accessToken, botToken string) (string, error) {
	var member BotMember
	err := client.callJSON(ctx, http.MethodPost, "/api/core/v1/bots/"+url.PathEscape(botToken)+"/secret", accessToken, nil, &member)
	return member.ClientSecret, err
}

// IssueBotAccessToken signs the bot member in with OAuth client credentials; its token lasts at most 15 minutes.
func (client *Client) IssueBotAccessToken(ctx context.Context, botToken, clientSecret string) (BotAccessToken, error) {
	form := url.Values{"grant_type": {"client_credentials"}}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL+"/api/core/v1/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return BotAccessToken{}, fmt.Errorf("build token request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpRequest.SetBasicAuth(botToken, clientSecret)
	var token struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	issuedAt := time.Now()
	if err := client.do(httpRequest, &token); err != nil {
		return BotAccessToken{}, err
	}
	return BotAccessToken{AccessToken: token.AccessToken, ExpiresAt: issuedAt.Add(time.Duration(token.ExpiresIn) * time.Second)}, nil
}

// GrantDelegatedSession asks core for a session the caller grants this module; the refresh token comes back once.
func (client *Client) GrantDelegatedSession(ctx context.Context, accessToken, module, purpose string) (SignIn, error) {
	var signIn SignIn
	err := client.callJSON(ctx, http.MethodPost, "/api/core/v1/delegated-sessions", accessToken,
		map[string]any{"module": module, "purpose": purpose}, &signIn)
	return signIn, err
}

// RefreshSession rotates a session's refresh token; core recomputes the permissions at every refresh.
func (client *Client) RefreshSession(ctx context.Context, refreshToken string) (SignIn, error) {
	var signIn SignIn
	err := client.callJSON(ctx, http.MethodPost, "/api/core/v1/auth/refresh", "", map[string]any{"refreshToken": refreshToken}, &signIn)
	return signIn, err
}

// EndSession revokes the session an access token belongs to.
func (client *Client) EndSession(ctx context.Context, accessToken string) error {
	return client.callJSON(ctx, http.MethodPost, "/api/core/v1/auth/logout", accessToken, nil, nil)
}

func (client *Client) callJSON(ctx context.Context, method, path, accessToken string, body any, target any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, method, client.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	if accessToken != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+accessToken)
	}
	return client.do(httpRequest, target)
}

func (client *Client) do(httpRequest *http.Request, target any) error {
	response, err := client.httpClient.Do(httpRequest)
	if err != nil {
		return errors.Join(ErrCoreUnavailable, err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return errors.Join(ErrCoreUnavailable, err)
	}
	if response.StatusCode >= http.StatusInternalServerError {
		return ErrCoreUnavailable
	}
	if response.StatusCode >= http.StatusBadRequest {
		return decodeRefusal(response.StatusCode, payload)
	}
	if target == nil || len(payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("decode core response: %w", err)
	}
	return nil
}

func decodeRefusal(status int, payload []byte) error {
	var body struct {
		Error struct {
			Code   string            `json:"code"`
			Fields map[string]string `json:"fields"`
		} `json:"error"`
		OAuthError string `json:"error_description"`
	}
	refusal := &RefusalError{Status: status, Code: "core_refused"}
	if json.Unmarshal(payload, &body) == nil && body.Error.Code != "" {
		refusal.Code, refusal.Fields = body.Error.Code, body.Error.Fields
	}
	return refusal
}
