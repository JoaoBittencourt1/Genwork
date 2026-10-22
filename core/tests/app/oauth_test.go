package app_test

import (
	"context"
	"crypto/sha256"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
)

func TestAuthorizationServerMetadata(t *testing.T) {
	app := newTestApp(t)

	resp := app.do(request{method: "GET", path: "/.well-known/oauth-authorization-server"})

	expectStatus(t, resp, http.StatusOK)
	var metadata map[string]any
	resp.decode(t, &metadata)
	expected := map[string]any{
		"issuer":                                testPublicURL,
		"authorization_endpoint":                testPublicURL + "/oauth/authorize",
		"token_endpoint":                        testPublicURL + "/api/core/v1/oauth/token",
		"registration_endpoint":                 testPublicURL + "/api/core/v1/oauth/register",
		"response_types_supported":              []any{"code"},
		"grant_types_supported":                 []any{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":      []any{"S256"},
		"token_endpoint_auth_methods_supported": []any{"none"},
		"scopes_supported":                      []any{"genwork"},
	}
	for key, want := range expected {
		if got := metadata[key]; !equalJSON(got, want) {
			t.Errorf("%s = %v, want %v", key, got, want)
		}
	}
}

func equalJSON(left, right any) bool {
	leftList, leftIsList := left.([]any)
	rightList, rightIsList := right.([]any)
	if leftIsList && rightIsList {
		if len(leftList) != len(rightList) {
			return false
		}
		for index := range leftList {
			if leftList[index] != rightList[index] {
				return false
			}
		}
		return true
	}
	return left == right
}

func TestClaudeRegistersAPublicClient(t *testing.T) {
	app := newTestApp(t)

	client := app.registerClient("Claude", claudeRedirectURI)

	if len(client.ClientID) != 22 || client.ClientName != "Claude" || client.TokenEndpointAuthMethod != "none" || client.ClientSecret != nil ||
		len(client.RedirectURIs) != 1 || client.RedirectURIs[0] != claudeRedirectURI || client.ClientIDIssuedAt != app.clock.Now().Unix() {
		t.Errorf("client = %+v", client)
	}
}

func TestRegistrationAcceptsLoopbackRedirectsAndUnknownMetadata(t *testing.T) {
	app := newTestApp(t)

	resp := app.do(request{method: "POST", path: "/api/core/v1/oauth/register", body: map[string]any{
		"redirect_uris": []string{"http://127.0.0.1:53682/callback", "http://localhost:3000/cb", "http://[::1]:8080/cb"},
		"logo_uri":      "https://example.com/logo.png", "software_id": "claude-code",
	}})

	expectStatus(t, resp, http.StatusCreated)
	var client registeredClientBody
	resp.decode(t, &client)
	if client.ClientName != "MCP client" || len(client.RedirectURIs) != 3 {
		t.Errorf("client = %+v", client)
	}
}

func TestRegistrationRejectsInsecureOrInvalidMetadata(t *testing.T) {
	app := newTestApp(t)
	sixURIs := []string{"https://a.example/1", "https://a.example/2", "https://a.example/3", "https://a.example/4", "https://a.example/5", "https://a.example/6"}
	cases := map[string]struct {
		body map[string]any
		code string
	}{
		"plain http":           {map[string]any{"redirect_uris": []string{"http://example.com/callback"}}, "invalid_redirect_uri"},
		"fragment":             {map[string]any{"redirect_uris": []string{"https://example.com/callback#token"}}, "invalid_redirect_uri"},
		"relative":             {map[string]any{"redirect_uris": []string{"/callback"}}, "invalid_redirect_uri"},
		"custom scheme":        {map[string]any{"redirect_uris": []string{"javascript:alert(1)"}}, "invalid_redirect_uri"},
		"no redirect":          {map[string]any{"redirect_uris": []string{}}, "invalid_redirect_uri"},
		"too many redirects":   {map[string]any{"redirect_uris": sixURIs}, "invalid_redirect_uri"},
		"name too long":        {map[string]any{"client_name": strings.Repeat("x", 101), "redirect_uris": []string{claudeRedirectURI}}, "invalid_client_metadata"},
		"client secret method": {map[string]any{"token_endpoint_auth_method": "client_secret_basic", "redirect_uris": []string{claudeRedirectURI}}, "invalid_client_metadata"},
		"implicit grant":       {map[string]any{"grant_types": []string{"implicit"}, "redirect_uris": []string{claudeRedirectURI}}, "invalid_client_metadata"},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			resp := app.do(request{method: "POST", path: "/api/core/v1/oauth/register", body: testCase.body})
			expectOAuthError(t, resp, http.StatusBadRequest, testCase.code)
		})
	}
}

func TestRegistrationIsRateLimitedPerAddress(t *testing.T) {
	limits := generousRateLimits
	limits.ClientRegistration = platform.RateLimit{PerMinute: 1, Burst: 2}
	app := newTestApp(t, withRateLimits(limits))
	body := map[string]any{"redirect_uris": []string{claudeRedirectURI}}

	for attempt := 0; attempt < 2; attempt++ {
		expectStatus(t, app.do(request{method: "POST", path: "/api/core/v1/oauth/register", body: body}), http.StatusCreated)
	}
	limited := app.do(request{method: "POST", path: "/api/core/v1/oauth/register", body: body})

	expectStatus(t, limited, http.StatusTooManyRequests)
}

func TestAuthorizationPreviewNamesTheClient(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "member")
	client := app.registerClient("Claude", claudeRedirectURI)
	ana := app.loginNative("ana@example.com")

	resp := app.previewAuthorization(ana.AccessToken, authorizationQuery(client.ClientID))

	expectStatus(t, resp, http.StatusOK)
	var body struct {
		ClientName  string `json:"clientName"`
		Scope       string `json:"scope"`
		RedirectURL string `json:"redirectUrl"`
	}
	resp.decode(t, &body)
	if body.ClientName != "Claude" || body.Scope != "genwork" || body.RedirectURL != "" {
		t.Errorf("preview = %+v", body)
	}
	expectError(t, app.do(request{method: "GET", path: "/api/core/v1/oauth/authorization-requests?" + authorizationQuery(client.ClientID).Encode()}),
		http.StatusUnauthorized, "invalid_token")
}

func TestAuthorizationErrorsThatMustNotRedirect(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "member")
	client := app.registerClient("Claude", claudeRedirectURI)
	ana := app.loginNative("ana@example.com")
	cases := map[string]struct {
		change func(url.Values)
		code   string
	}{
		"unknown client":           {func(query url.Values) { query.Set("client_id", "unknown-client-0123456") }, "invalid_client"},
		"unregistered redirect":    {func(query url.Values) { query.Set("redirect_uri", "https://evil.example/callback") }, "invalid_redirect_uri"},
		"missing redirect":         {func(query url.Values) { query.Del("redirect_uri") }, "invalid_redirect_uri"},
		"prefix of the registered": {func(query url.Values) { query.Set("redirect_uri", claudeRedirectURI+"/more") }, "invalid_redirect_uri"},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			query := authorizationQuery(client.ClientID)
			testCase.change(query)

			expectError(t, app.previewAuthorization(ana.AccessToken, query), http.StatusBadRequest, testCase.code)
			expectError(t, app.decideAuthorization(ana.AccessToken, query, "allow"), http.StatusBadRequest, testCase.code)
		})
	}
}

func TestAuthorizationErrorsRedirectWithTheState(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "member")
	client := app.registerClient("Claude", claudeRedirectURI)
	ana := app.loginNative("ana@example.com")
	cases := map[string]struct {
		change func(url.Values)
		code   string
	}{
		"implicit flow":  {func(query url.Values) { query.Set("response_type", "token") }, "unsupported_response_type"},
		"missing PKCE":   {func(query url.Values) { query.Del("code_challenge") }, "invalid_request"},
		"plain PKCE":     {func(query url.Values) { query.Set("code_challenge_method", "plain") }, "invalid_request"},
		"malformed PKCE": {func(query url.Values) { query.Set("code_challenge", "short") }, "invalid_request"},
		"other scope":    {func(query url.Values) { query.Set("scope", "admin") }, "invalid_scope"},
		"other resource": {func(query url.Values) { query.Set("resource", "https://other.example/mcp") }, "invalid_target"},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			query := authorizationQuery(client.ClientID)
			testCase.change(query)

			for _, resp := range []response{app.previewAuthorization(ana.AccessToken, query), app.decideAuthorization(ana.AccessToken, query, "allow")} {
				target, parameters := redirectParameters(t, resp)
				if target.Scheme+"://"+target.Host+target.Path != claudeRedirectURI || parameters.Get("error") != testCase.code ||
					parameters.Get("state") != "state-from-claude" || parameters.Get("iss") != testPublicURL || parameters.Get("code") != "" {
					t.Errorf("redirect = %s", target)
				}
			}
		})
	}
}

func TestAuthorizationAcceptsAnOmittedScopeAndResource(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "member")
	client := app.registerClient("Claude", claudeRedirectURI)
	ana := app.loginNative("ana@example.com")
	query := authorizationQuery(client.ClientID)
	query.Del("scope")
	query.Del("resource")

	_, parameters := redirectParameters(t, app.decideAuthorization(ana.AccessToken, query, "allow"))

	if parameters.Get("code") == "" {
		t.Errorf("parameters = %v", parameters)
	}
}

func TestConsentApprovalRedirectsWithACodeTheStateAndTheIssuer(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "member")
	client := app.registerClient("Claude", claudeRedirectURI)
	ana := app.loginNative("ana@example.com")

	target, parameters := redirectParameters(t, app.decideAuthorization(ana.AccessToken, authorizationQuery(client.ClientID), "allow"))

	if target.Scheme+"://"+target.Host+target.Path != claudeRedirectURI || len(parameters.Get("code")) < 43 ||
		parameters.Get("state") != "state-from-claude" || parameters.Get("iss") != testPublicURL {
		t.Errorf("redirect = %s", target)
	}
}

func TestConsentDenialRedirectsWithoutACode(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "member")
	client := app.registerClient("Claude", claudeRedirectURI)
	ana := app.loginNative("ana@example.com")

	_, parameters := redirectParameters(t, app.decideAuthorization(ana.AccessToken, authorizationQuery(client.ClientID), "deny"))

	if parameters.Get("error") != "access_denied" || parameters.Get("state") != "state-from-claude" || parameters.Get("code") != "" {
		t.Errorf("parameters = %v", parameters)
	}
	if count := app.countAuthorizationCodes(); count != 0 {
		t.Errorf("codes = %d after a denial", count)
	}
	expectError(t, app.decideAuthorization(ana.AccessToken, authorizationQuery(client.ClientID), "maybe"), http.StatusBadRequest, "validation_failed")
}

func (app *testApp) countAuthorizationCodes() int {
	app.t.Helper()
	var count int
	if err := app.db.QueryRow(context.Background(), `SELECT count(*) FROM oauth_authorization_codes`).Scan(&count); err != nil {
		app.t.Fatalf("count codes: %v", err)
	}
	return count
}

func TestAuthorizationCodesAreStoredOnlyAsHashes(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "member")
	client := app.registerClient("Claude", claudeRedirectURI)
	ana := app.loginNative("ana@example.com")

	code := app.authorizedCode(ana.AccessToken, client.ClientID)

	var stored []byte
	var row string
	err := app.db.QueryRow(context.Background(), `SELECT code_hash, oauth_authorization_codes::text FROM oauth_authorization_codes`).Scan(&stored, &row)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(code))
	if string(stored) != string(digest[:]) || strings.Contains(row, code) {
		t.Errorf("the code must be stored as its SHA-256 only")
	}
}

func TestCodeExchangeIssuesTokensCoreAccepts(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "member")
	client := app.registerClient("Claude", claudeRedirectURI)
	browser := app.loginNative("ana@example.com")
	code := app.authorizedCode(browser.AccessToken, client.ClientID)

	resp := app.requestToken(codeExchange(client.ClientID, code))

	expectStatus(t, resp, http.StatusOK)
	if resp.header.Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q", resp.header.Get("Cache-Control"))
	}
	var tokens tokenBody
	resp.decode(t, &tokens)
	if tokens.TokenType != "Bearer" || tokens.ExpiresIn != 900 || tokens.Scope != "genwork" || tokens.RefreshToken == "" {
		t.Errorf("tokens = %+v", tokens)
	}
	if me := app.meBody(tokens.AccessToken); !slices.Contains(me.Permissions, "core.user.read") {
		t.Errorf("the token must carry the member's own permissions: %+v", me)
	}
}

func TestCodeExchangeRequiresTheMatchingVerifierClientAndRedirect(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "member")
	client := app.registerClient("Claude", claudeRedirectURI, "https://claude.ai/other")
	other := app.registerClient("Other", claudeRedirectURI)
	browser := app.loginNative("ana@example.com")
	code := app.authorizedCode(browser.AccessToken, client.ClientID)
	cases := map[string]func(url.Values){
		"wrong verifier": func(form url.Values) { form.Set("code_verifier", strings.Repeat("a", 43)) },
		"other client":   func(form url.Values) { form.Set("client_id", other.ClientID) },
		"other redirect": func(form url.Values) { form.Set("redirect_uri", "https://claude.ai/other") },
		"unknown code":   func(form url.Values) { form.Set("code", "unknown-code") },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			form := codeExchange(client.ClientID, code)
			change(form)

			resp := app.requestToken(form)

			expectOAuthError(t, resp, http.StatusBadRequest, "invalid_grant")
			if resp.header.Get("Cache-Control") != "no-store" {
				t.Errorf("Cache-Control = %q", resp.header.Get("Cache-Control"))
			}
		})
	}
	expectStatus(t, app.requestToken(codeExchange(client.ClientID, code)), http.StatusOK)
}

func TestCodesExpireAfterFiveMinutes(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "member")
	client := app.registerClient("Claude", claudeRedirectURI)
	browser := app.loginNative("ana@example.com")
	code := app.authorizedCode(browser.AccessToken, client.ClientID)

	app.clock.Advance(5*time.Minute + time.Second)

	expectOAuthError(t, app.requestToken(codeExchange(client.ClientID, code)), http.StatusBadRequest, "invalid_grant")
}

func TestReusedCodeRevokesTheSessionOfTheFirstExchange(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "member")
	client := app.registerClient("Claude", claudeRedirectURI)
	browser := app.loginNative("ana@example.com")
	code := app.authorizedCode(browser.AccessToken, client.ClientID)
	first := app.exchangeCode(client.ClientID, code)

	expectOAuthError(t, app.requestToken(codeExchange(client.ClientID, code)), http.StatusBadRequest, "invalid_grant")

	expectError(t, app.do(request{method: "GET", path: "/api/core/v1/me", accessToken: first.AccessToken}), http.StatusUnauthorized, "invalid_token")
	expectOAuthError(t, app.requestToken(refreshGrant(client.ClientID, first.RefreshToken)), http.StatusBadRequest, "invalid_grant")
}

func TestRefreshRotatesAndAReusedTokenRevokesTheSession(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "member")
	client, _, first := app.connectClaude("ana@example.com")

	resp := app.requestToken(refreshGrant(client.ClientID, first.RefreshToken))

	expectStatus(t, resp, http.StatusOK)
	var rotated tokenBody
	resp.decode(t, &rotated)
	if rotated.RefreshToken == "" || rotated.RefreshToken == first.RefreshToken || rotated.AccessToken == "" || rotated.Scope != "genwork" {
		t.Fatalf("rotated = %+v", rotated)
	}
	expectOAuthError(t, app.requestToken(refreshGrant(client.ClientID, first.RefreshToken)), http.StatusBadRequest, "invalid_grant")
	expectOAuthError(t, app.requestToken(refreshGrant(client.ClientID, rotated.RefreshToken)), http.StatusBadRequest, "invalid_grant")
}

func TestRefreshByAnotherClientRotatesNothing(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "member")
	client, _, first := app.connectClaude("ana@example.com")
	other := app.registerClient("Other", claudeRedirectURI)

	expectOAuthError(t, app.requestToken(refreshGrant(other.ClientID, first.RefreshToken)), http.StatusBadRequest, "invalid_grant")

	expectStatus(t, app.requestToken(refreshGrant(client.ClientID, first.RefreshToken)), http.StatusOK)
}

func TestOAuthRefreshTokensDoNotWorkForTheAppSignIn(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "member")
	client, _, first := app.connectClaude("ana@example.com")

	resp := app.do(request{method: "POST", path: "/api/core/v1/auth/refresh", body: map[string]any{"refreshToken": first.RefreshToken}})

	expectError(t, resp, http.StatusUnauthorized, "invalid_token")
	expectStatus(t, app.requestToken(refreshGrant(client.ClientID, first.RefreshToken)), http.StatusOK)
}

func TestAssistantGrantsAreRevocableSessionsNamedAfterTheClient(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "member")
	client, browser, claude := app.connectClaude("ana@example.com")

	list := app.do(request{method: "GET", path: "/api/core/v1/auth/sessions", accessToken: browser.AccessToken})
	expectStatus(t, list, http.StatusOK)
	var body struct {
		Sessions []struct {
			Token      string  `json:"token"`
			ClientType string  `json:"clientType"`
			DeviceName *string `json:"deviceName"`
		} `json:"sessions"`
	}
	list.decode(t, &body)
	var claudeSession string
	for _, session := range body.Sessions {
		if session.ClientType == "oauth" && session.DeviceName != nil && *session.DeviceName == "Claude" {
			claudeSession = session.Token
		}
	}
	if claudeSession == "" {
		t.Fatalf("sessions = %+v", body.Sessions)
	}

	expectStatus(t, app.do(request{method: "DELETE", path: "/api/core/v1/auth/sessions/" + claudeSession, accessToken: browser.AccessToken}), http.StatusNoContent)

	expectOAuthError(t, app.requestToken(refreshGrant(client.ClientID, claude.RefreshToken)), http.StatusBadRequest, "invalid_grant")
}

func TestADeactivatedMemberCannotExchangeOrRefresh(t *testing.T) {
	app := newTestApp(t)
	bruno := app.createUser("bruno@example.com", "Bruno", "member")
	client := app.registerClient("Claude", claudeRedirectURI)
	browser := app.loginNative("bruno@example.com")
	pendingCode := app.authorizedCode(browser.AccessToken, client.ClientID)
	connected := app.exchangeCode(client.ClientID, app.authorizedCode(browser.AccessToken, client.ClientID))

	admin := app.signedInAdmin()
	expectStatus(t, app.patchMember(admin.AccessToken, bruno.Token, map[string]any{"status": "deactivated"}), http.StatusOK)

	expectOAuthError(t, app.requestToken(codeExchange(client.ClientID, pendingCode)), http.StatusBadRequest, "invalid_grant")
	expectOAuthError(t, app.requestToken(refreshGrant(client.ClientID, connected.RefreshToken)), http.StatusBadRequest, "invalid_grant")
}

func TestTokenEndpointRejectsMalformedRequests(t *testing.T) {
	app := newTestApp(t)
	cases := map[string]struct {
		form url.Values
		code string
	}{
		"unknown grant":         {url.Values{"grant_type": {"password"}}, "unsupported_grant_type"},
		"no grant":              {url.Values{}, "invalid_request"},
		"code without verifier": {url.Values{"grant_type": {"authorization_code"}, "code": {"x"}, "client_id": {"y"}, "redirect_uri": {claudeRedirectURI}}, "invalid_request"},
		"refresh without token": {url.Values{"grant_type": {"refresh_token"}, "client_id": {"y"}}, "invalid_request"},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			expectOAuthError(t, app.requestToken(testCase.form), http.StatusBadRequest, testCase.code)
		})
	}
	jsonBody := app.do(request{method: "POST", path: "/api/core/v1/oauth/token", body: map[string]any{"grant_type": "refresh_token"}})
	expectOAuthError(t, jsonBody, http.StatusBadRequest, "invalid_request")
}

func TestTokenEndpointIsRateLimitedPerAddress(t *testing.T) {
	limits := generousRateLimits
	limits.OAuthToken = platform.RateLimit{PerMinute: 1, Burst: 1}
	app := newTestApp(t, withRateLimits(limits))

	app.requestToken(url.Values{"grant_type": {"password"}})
	limited := app.requestToken(url.Values{"grant_type": {"password"}})

	expectStatus(t, limited, http.StatusTooManyRequests)
}
