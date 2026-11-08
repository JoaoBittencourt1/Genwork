package app_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/access"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/mcpserverstest"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runs"
)

type oauthServerBody struct {
	Token string `json:"token"`
	OAuth *struct {
		Status    string     `json:"status"`
		ClientID  *string    `json:"clientId"`
		ExpiresAt *time.Time `json:"expiresAt"`
		LastError *string    `json:"lastError"`
	} `json:"oauth"`
}

func (app *testApp) createOAuthServer(fake *mcpserverstest.FakeAuthorizationServer) mcpServerBody {
	app.t.Helper()
	resp := app.createMCPServer(map[string]any{"name": "linear", "transport": "http", "url": fake.MCPURL()})
	expectStatus(app.t, resp, http.StatusCreated)
	var server mcpServerBody
	resp.decode(app.t, &server)
	return server
}

func (app *testApp) startOAuth(serverToken string, body map[string]any) response {
	app.t.Helper()
	if body == nil {
		body = map[string]any{}
	}
	return app.do(request{method: "POST", path: "/api/genagents/v1/mcp-servers/" + serverToken + "/oauth/authorizations", body: body})
}

// authorizationOf starts an authorization and returns its URL's query.
func (app *testApp) authorizationOf(serverToken string, body map[string]any) url.Values {
	app.t.Helper()
	resp := app.startOAuth(serverToken, body)
	expectStatus(app.t, resp, http.StatusCreated)
	var started struct {
		AuthorizationURL string `json:"authorizationUrl"`
	}
	resp.decode(app.t, &started)
	parsed, err := url.Parse(started.AuthorizationURL)
	if err != nil {
		app.t.Fatalf("parse %q: %v", started.AuthorizationURL, err)
	}
	return parsed.Query()
}

func (app *testApp) oauthCallback(query url.Values) response {
	app.t.Helper()
	return app.do(request{method: "GET", path: "/api/genagents/v1/mcp-servers/oauth/callback?" + query.Encode(), anonymous: true})
}

// connectOAuth runs the whole browser round trip: start, consent, callback.
func (app *testApp) connectOAuth(fake *mcpserverstest.FakeAuthorizationServer, serverToken string) {
	app.t.Helper()
	authorization := app.authorizationOf(serverToken, nil)
	code := fake.Approve(authorization.Get("code_challenge"))
	resp := app.oauthCallback(url.Values{"state": {authorization.Get("state")}, "code": {code}})
	expectStatus(app.t, resp, http.StatusSeeOther)
	if resp.header("Location") != testPublicURL+"/ai/mcp-servers?mcpOAuth=connected" {
		app.t.Fatalf("callback went to %q", resp.header("Location"))
	}
}

func (app *testApp) oauthServer(token string) oauthServerBody {
	app.t.Helper()
	var body struct {
		Servers []oauthServerBody `json:"servers"`
	}
	app.do(request{method: "GET", path: "/api/genagents/v1/mcp-servers"}).decode(app.t, &body)
	for _, server := range body.Servers {
		if server.Token == token {
			return server
		}
	}
	app.t.Fatalf("server %s not found", token)
	return oauthServerBody{}
}

// claimedAuthorization claims a run of a bot that uses the server and returns the Authorization header it carries.
func (app *testApp) claimedAuthorization(serverToken string) string {
	app.t.Helper()
	runner := app.registerRunner("box-"+strings.ToLower(time.Now().Format("150405.000000")), heartbeatOptions{tools: []string{"claude"}, github: true})
	bot := app.createBotOn(app.createAPIKeyAccount("Claude "+runner.Name, "anthropic", "sk-ant-oauth-test").Token, func(body map[string]any) {
		body["name"] = "Bot " + runner.Name
		body["kind"] = "assistant"
		body["mcpServerTokens"] = []string{serverToken}
	})
	app.queueRun(bot.Token, runs.RunKindChat, "task-"+runner.Name)
	claimed, resp := app.claim(runner, 1)
	if claimed == nil {
		app.t.Fatalf("no claim: %s", resp.body)
	}
	for _, server := range claimed.MCPServers {
		for _, header := range server.Headers {
			if header.Name == "Authorization" {
				return header.Value
			}
		}
	}
	return ""
}

func TestConnectingAnMCPServerRunsTheOAuthFlowWithPKCE(t *testing.T) {
	app := newTestApp(t)
	fake := mcpserverstest.NewFakeAuthorizationServer(t)
	server := app.createOAuthServer(fake)

	authorization := app.authorizationOf(server.Token, nil)

	if authorization.Get("client_id") != mcpserverstest.RegisteredClientID || authorization.Get("code_challenge_method") != "S256" ||
		authorization.Get("resource") != fake.MCPURL() || authorization.Get("scope") != "mcp:tools" ||
		authorization.Get("redirect_uri") != testPublicURL+"/api/genagents/v1/mcp-servers/oauth/callback" || authorization.Get("state") == "" {
		t.Fatalf("authorization = %v", authorization)
	}
	code := fake.Approve(authorization.Get("code_challenge"))
	expectStatus(t, app.oauthCallback(url.Values{"state": {authorization.Get("state")}, "code": {code}}), http.StatusSeeOther)
	got := app.oauthServer(server.Token)
	if got.OAuth == nil || got.OAuth.Status != "connected" || got.OAuth.ClientID == nil || got.OAuth.ExpiresAt == nil {
		t.Fatalf("server = %+v", got)
	}
	tokens := fake.Tokens()
	if len(tokens) != 1 || tokens[0]["resource"] != fake.MCPURL() || tokens[0]["client_id"] != mcpserverstest.RegisteredClientID {
		t.Errorf("token requests = %v", tokens)
	}
}

func TestOAuthTokensAreNeverReturnedAndStoredEncrypted(t *testing.T) {
	app := newTestApp(t)
	fake := mcpserverstest.NewFakeAuthorizationServer(t)
	server := app.createOAuthServer(fake)

	app.connectOAuth(fake, server.Token)

	listing := app.do(request{method: "GET", path: "/api/genagents/v1/mcp-servers"})
	if strings.Contains(string(listing.body), "mcp-access-1") || strings.Contains(string(listing.body), "mcp-refresh-1") {
		t.Errorf("the listing carries a token: %s", listing.body)
	}
	var access, refresh []byte
	err := app.db.QueryRow(context.Background(), `SELECT access_token_ciphertext, refresh_token_ciphertext FROM mcp_server_oauth_connections`).Scan(&access, &refresh)
	if err != nil {
		t.Fatalf("read tokens: %v", err)
	}
	if strings.Contains(string(access), "mcp-access-1") || strings.Contains(string(refresh), "mcp-refresh-1") || len(access) == 0 {
		t.Error("the tokens are not stored encrypted")
	}
}

func TestARunGetsTheBearerTokenAndARefreshedOneBeforeExpiry(t *testing.T) {
	app := newTestApp(t)
	fake := mcpserverstest.NewFakeAuthorizationServer(t)
	server := app.createOAuthServer(fake)
	app.connectOAuth(fake, server.Token)

	if got := app.claimedAuthorization(server.Token); got != "Bearer mcp-access-1" {
		t.Errorf("first run header = %q", got)
	}
	app.clock.Advance(56 * time.Minute)

	if got := app.claimedAuthorization(server.Token); got != "Bearer mcp-access-2" {
		t.Errorf("header after refresh = %q", got)
	}
	tokens := fake.Tokens()
	if last := tokens[len(tokens)-1]; last["grant_type"] != "refresh_token" || last["resource"] != fake.MCPURL() {
		t.Errorf("refresh request = %v", last)
	}
}

func TestAFailedRefreshMarksTheConnectionAndTheRunStartsWithoutIt(t *testing.T) {
	app := newTestApp(t)
	fake := mcpserverstest.NewFakeAuthorizationServer(t)
	server := app.createOAuthServer(fake)
	app.connectOAuth(fake, server.Token)
	fake.FailRefresh = true
	app.clock.Advance(56 * time.Minute)

	if got := app.claimedAuthorization(server.Token); got != "" {
		t.Errorf("header = %q", got)
	}
	if got := app.oauthServer(server.Token); got.OAuth.Status != "error" || got.OAuth.LastError == nil || *got.OAuth.LastError != "refresh_failed" {
		t.Errorf("server = %+v", got)
	}
}

func TestAStateWorksOnlyOnce(t *testing.T) {
	app := newTestApp(t)
	fake := mcpserverstest.NewFakeAuthorizationServer(t)
	server := app.createOAuthServer(fake)
	authorization := app.authorizationOf(server.Token, nil)
	callback := url.Values{"state": {authorization.Get("state")}, "code": {fake.Approve(authorization.Get("code_challenge"))}}
	expectStatus(t, app.oauthCallback(callback), http.StatusSeeOther)

	replay := app.oauthCallback(callback)

	if location := replay.header("Location"); location != testPublicURL+"/ai/mcp-servers?mcpOAuth=failed&reason=invalid_state" {
		t.Errorf("replay went to %q", location)
	}
	if len(fake.Tokens()) != 1 {
		t.Errorf("the replay exchanged the code again: %v", fake.Tokens())
	}
}

func TestAnExpiredOrUnknownStateIsRefused(t *testing.T) {
	app := newTestApp(t)
	fake := mcpserverstest.NewFakeAuthorizationServer(t)
	server := app.createOAuthServer(fake)
	authorization := app.authorizationOf(server.Token, nil)
	app.clock.Advance(11 * time.Minute)

	expired := app.oauthCallback(url.Values{"state": {authorization.Get("state")}, "code": {fake.Approve(authorization.Get("code_challenge"))}})
	unknown := app.oauthCallback(url.Values{"state": {"made-up"}, "code": {"x"}})

	for _, resp := range []response{expired, unknown} {
		if !strings.Contains(resp.header("Location"), "reason=invalid_state") {
			t.Errorf("callback went to %q", resp.header("Location"))
		}
	}
}

func TestADeniedAuthorizationShowsAsAnError(t *testing.T) {
	app := newTestApp(t)
	fake := mcpserverstest.NewFakeAuthorizationServer(t)
	server := app.createOAuthServer(fake)
	authorization := app.authorizationOf(server.Token, nil)

	resp := app.oauthCallback(url.Values{"state": {authorization.Get("state")}, "error": {"access_denied"}})

	if !strings.Contains(resp.header("Location"), "mcpOAuth=failed&reason=authorization_denied") {
		t.Errorf("callback went to %q", resp.header("Location"))
	}
	if got := app.oauthServer(server.Token); got.OAuth.Status != "error" || *got.OAuth.LastError != "authorization_denied" {
		t.Errorf("server = %+v", got)
	}
}

func TestAServerWithoutRegistrationNeedsAClientID(t *testing.T) {
	app := newTestApp(t)
	fake := mcpserverstest.NewFakeAuthorizationServer(t)
	fake.WithoutRegistration = true
	server := app.createOAuthServer(fake)

	expectError(t, app.startOAuth(server.Token, nil), http.StatusConflict, "oauth_client_id_required")
	authorization := app.authorizationOf(server.Token, map[string]any{"clientId": mcpserverstest.ManualClientID, "clientSecret": mcpserverstest.ManualClientSecret})
	code := fake.Approve(authorization.Get("code_challenge"))
	expectStatus(t, app.oauthCallback(url.Values{"state": {authorization.Get("state")}, "code": {code}}), http.StatusSeeOther)

	if authorization.Get("client_id") != mcpserverstest.ManualClientID || fake.Registrations != 0 {
		t.Errorf("authorization = %v, registrations = %d", authorization, fake.Registrations)
	}
	if len(fake.BasicAuthClients) != 1 {
		t.Errorf("the confidential client did not authenticate with HTTP Basic")
	}
	listing := app.do(request{method: "GET", path: "/api/genagents/v1/mcp-servers"})
	if strings.Contains(string(listing.body), mcpserverstest.ManualClientSecret) {
		t.Error("the listing carries the client secret")
	}
}

func TestOAuthNeedsAnHTTPServerAndThePermission(t *testing.T) {
	app := newTestApp(t)
	stdio := app.createMCPServer(map[string]any{"name": "local", "transport": "stdio", "command": "npx", "args": []string{"server"}})
	var server mcpServerBody
	stdio.decode(t, &server)

	expectValidationError(t, app.startOAuth(server.Token, nil), "transport")
	resp := app.do(request{method: "POST", path: "/api/genagents/v1/mcp-servers/" + server.Token + "/oauth/authorizations", body: map[string]any{},
		accessToken: app.tokenWith(access.PermissionBotRead)})
	expectStatus(t, resp, http.StatusForbidden)
	expectValidationError(t, app.startOAuth(server.Token, map[string]any{"clientSecret": "secret-without-client"}), "clientSecret")
}

func TestAnUnreachableAuthorizationServerFailsDiscovery(t *testing.T) {
	app := newTestApp(t)
	resp := app.createMCPServer(map[string]any{"name": "plain", "transport": "http", "url": "http://127.0.0.1:1/mcp"})
	var server mcpServerBody
	resp.decode(t, &server)

	expectError(t, app.startOAuth(server.Token, nil), http.StatusBadGateway, "oauth_discovery_failed")
}

func TestDisconnectingRemovesTheTokensFromRuns(t *testing.T) {
	app := newTestApp(t)
	fake := mcpserverstest.NewFakeAuthorizationServer(t)
	server := app.createOAuthServer(fake)
	app.connectOAuth(fake, server.Token)

	expectNoContent(t, app.do(request{method: "DELETE", path: "/api/genagents/v1/mcp-servers/" + server.Token + "/oauth"}))

	if got := app.oauthServer(server.Token); got.OAuth != nil {
		t.Errorf("server = %+v", got)
	}
	if got := app.claimedAuthorization(server.Token); got != "" {
		t.Errorf("header after disconnect = %q", got)
	}
	expectError(t, app.do(request{method: "DELETE", path: "/api/genagents/v1/mcp-servers/" + server.Token + "/oauth"}), http.StatusNotFound, "oauth_not_connected")
}

func TestChangingTheServerURLDropsItsOAuthTokens(t *testing.T) {
	app := newTestApp(t)
	fake := mcpserverstest.NewFakeAuthorizationServer(t)
	server := app.createOAuthServer(fake)
	app.connectOAuth(fake, server.Token)
	pending := app.authorizationOf(server.Token, nil)

	resp := app.do(request{method: "PATCH", path: "/api/genagents/v1/mcp-servers/" + server.Token, body: map[string]any{"url": "https://other-provider.example/mcp"}})

	expectStatus(t, resp, http.StatusOK)
	if got := app.oauthServer(server.Token); got.OAuth != nil {
		t.Errorf("the connection survived the URL change: %+v", got.OAuth)
	}
	if got := app.claimedAuthorization(server.Token); got != "" {
		t.Errorf("the old provider's token went to the new URL: %q", got)
	}
	callback := app.oauthCallback(url.Values{"state": {pending.Get("state")}, "code": {fake.Approve(pending.Get("code_challenge"))}})
	if !strings.Contains(callback.header("Location"), "reason=invalid_state") {
		t.Errorf("an authorization started before the change completed: %q", callback.header("Location"))
	}
}

func TestATokenIssuedForAnotherURLIsNeverAttached(t *testing.T) {
	app := newTestApp(t)
	fake := mcpserverstest.NewFakeAuthorizationServer(t)
	server := app.createOAuthServer(fake)
	app.connectOAuth(fake, server.Token)
	if _, err := app.db.Exec(context.Background(), `UPDATE mcp_servers SET config = jsonb_set(config, '{url}', '"https://elsewhere.example/mcp"') WHERE token = $1`, server.Token); err != nil {
		t.Fatal(err)
	}

	if got := app.claimedAuthorization(server.Token); got != "" {
		t.Errorf("header = %q", got)
	}
	if got := app.oauthServer(server.Token); got.OAuth == nil || got.OAuth.Status != "error" || *got.OAuth.LastError != "server_url_changed" {
		t.Errorf("server = %+v", got.OAuth)
	}
}

func TestAnUnfinishedReconnectionKeepsTheActiveClientAndTokens(t *testing.T) {
	app := newTestApp(t)
	fake := mcpserverstest.NewFakeAuthorizationServer(t)
	server := app.createOAuthServer(fake)
	app.connectOAuth(fake, server.Token)
	app.authorizationOf(server.Token, map[string]any{"clientId": mcpserverstest.ManualClientID, "clientSecret": mcpserverstest.ManualClientSecret})
	app.clock.Advance(56 * time.Minute)

	if got := app.claimedAuthorization(server.Token); got != "Bearer mcp-access-2" {
		t.Errorf("header after refresh = %q", got)
	}
	tokens := fake.Tokens()
	if last := tokens[len(tokens)-1]; last["grant_type"] != "refresh_token" || last["client_id"] != mcpserverstest.RegisteredClientID {
		t.Errorf("the refresh did not use the active client: %v", last)
	}
	if got := app.oauthServer(server.Token); got.OAuth.Status != "connected" || *got.OAuth.ClientID != mcpserverstest.RegisteredClientID {
		t.Errorf("server = %+v", got.OAuth)
	}
}

func TestAReconnectionReplacesClientAndTokensOnlyWhenItSucceeds(t *testing.T) {
	app := newTestApp(t)
	fake := mcpserverstest.NewFakeAuthorizationServer(t)
	server := app.createOAuthServer(fake)
	app.connectOAuth(fake, server.Token)
	denied := app.authorizationOf(server.Token, nil)
	app.oauthCallback(url.Values{"state": {denied.Get("state")}, "error": {"access_denied"}})
	if got := app.oauthServer(server.Token); got.OAuth.Status != "connected" {
		t.Fatalf("an abandoned reconnection broke the connection: %+v", got.OAuth)
	}

	manual := app.authorizationOf(server.Token, map[string]any{"clientId": mcpserverstest.ManualClientID, "clientSecret": mcpserverstest.ManualClientSecret})
	expectStatus(t, app.oauthCallback(url.Values{"state": {manual.Get("state")}, "code": {fake.Approve(manual.Get("code_challenge"))}}), http.StatusSeeOther)

	if got := app.oauthServer(server.Token); got.OAuth.Status != "connected" || *got.OAuth.ClientID != mcpserverstest.ManualClientID {
		t.Errorf("server = %+v", got.OAuth)
	}
}
