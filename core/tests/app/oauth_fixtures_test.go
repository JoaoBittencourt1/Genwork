package app_test

import (
	"net/http"
	"net/url"
	"testing"
)

// The PKCE pair from RFC 7636, appendix B.
const (
	testCodeVerifier  = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	testCodeChallenge = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	claudeRedirectURI = "https://claude.ai/api/mcp/auth_callback"
	testMCPResource   = testPublicURL + "/mcp"
)

type registeredClientBody struct {
	ClientID                string   `json:"client_id"`
	ClientIDIssuedAt        int64    `json:"client_id_issued_at"`
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	ClientSecret            *string  `json:"client_secret"`
}

type oauthErrorBody struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type tokenBody struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

type redirectBody struct {
	RedirectURL string `json:"redirectUrl"`
}

func (app *testApp) registerClient(name string, redirectURIs ...string) registeredClientBody {
	app.t.Helper()
	resp := app.do(request{method: "POST", path: "/api/core/v1/oauth/register", body: map[string]any{
		"client_name": name, "redirect_uris": redirectURIs,
		"grant_types": []string{"authorization_code", "refresh_token"}, "response_types": []string{"code"},
		"token_endpoint_auth_method": "none",
	}})
	expectStatus(app.t, resp, http.StatusCreated)
	var body registeredClientBody
	resp.decode(app.t, &body)
	return body
}

// authorizationQuery is a valid request from Claude; tests change single values to make it invalid.
func authorizationQuery(clientID string) url.Values {
	return url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {claudeRedirectURI},
		"state":                 {"state-from-claude"},
		"code_challenge":        {testCodeChallenge},
		"code_challenge_method": {"S256"},
		"scope":                 {"genwork"},
		"resource":              {testMCPResource},
	}
}

// decisionBody is the consent page's request for the same parameters.
func decisionBody(query url.Values, decision string) map[string]any {
	return map[string]any{
		"responseType": query.Get("response_type"), "clientId": query.Get("client_id"), "redirectUri": query.Get("redirect_uri"),
		"state": query.Get("state"), "codeChallenge": query.Get("code_challenge"), "codeChallengeMethod": query.Get("code_challenge_method"),
		"scope": query.Get("scope"), "resource": query.Get("resource"), "decision": decision,
	}
}

func (app *testApp) previewAuthorization(accessToken string, query url.Values) response {
	app.t.Helper()
	return app.do(request{method: "GET", path: "/api/core/v1/oauth/authorization-requests?" + query.Encode(), accessToken: accessToken})
}

func (app *testApp) decideAuthorization(accessToken string, query url.Values, decision string) response {
	app.t.Helper()
	return app.do(request{method: "POST", path: "/api/core/v1/oauth/authorizations", body: decisionBody(query, decision), accessToken: accessToken})
}

// redirectParameters follows the URL core answered with, as the consent page sends the browser there.
func redirectParameters(t *testing.T, resp response) (*url.URL, url.Values) {
	t.Helper()
	expectStatus(t, resp, http.StatusOK)
	var body redirectBody
	resp.decode(t, &body)
	target, err := url.Parse(body.RedirectURL)
	if err != nil || body.RedirectURL == "" {
		t.Fatalf("redirect URL = %q (%v)", body.RedirectURL, err)
	}
	return target, target.Query()
}

// authorizedCode runs the consent as user and returns the code Claude receives.
func (app *testApp) authorizedCode(accessToken, clientID string) string {
	app.t.Helper()
	_, parameters := redirectParameters(app.t, app.decideAuthorization(accessToken, authorizationQuery(clientID), "allow"))
	if parameters.Get("code") == "" {
		app.t.Fatalf("no code in %v", parameters)
	}
	return parameters.Get("code")
}

func (app *testApp) requestToken(form url.Values) response {
	app.t.Helper()
	return app.do(request{method: "POST", path: "/api/core/v1/oauth/token", form: form})
}

func codeExchange(clientID, code string) url.Values {
	return url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {claudeRedirectURI},
		"client_id": {clientID}, "code_verifier": {testCodeVerifier},
	}
}

func refreshGrant(clientID, refreshToken string) url.Values {
	return url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refreshToken}, "client_id": {clientID}}
}

func (app *testApp) exchangeCode(clientID, code string) tokenBody {
	app.t.Helper()
	resp := app.requestToken(codeExchange(clientID, code))
	expectStatus(app.t, resp, http.StatusOK)
	var body tokenBody
	resp.decode(app.t, &body)
	return body
}

// connectClaude registers Claude, signs ana in, and completes the whole flow.
func (app *testApp) connectClaude(email string) (registeredClientBody, signInBody, tokenBody) {
	app.t.Helper()
	client := app.registerClient("Claude", claudeRedirectURI)
	browser := app.loginNative(email)
	return client, browser, app.exchangeCode(client.ClientID, app.authorizedCode(browser.AccessToken, client.ClientID))
}

func expectOAuthError(t *testing.T, resp response, status int, code string) {
	t.Helper()
	expectStatus(t, resp, status)
	var body oauthErrorBody
	resp.decode(t, &body)
	if body.Error != code {
		t.Fatalf("error = %q, want %q; body = %s", body.Error, code, resp.body)
	}
}
