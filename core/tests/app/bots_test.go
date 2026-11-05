package app_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
)

type botBody struct {
	memberBody
	Kind         string `json:"kind"`
	ClientSecret string `json:"clientSecret"`
}

func (app *testApp) createBot(accessToken, name, roleName string) botBody {
	app.t.Helper()
	resp := app.do(request{method: "POST", path: "/api/core/v1/bots", accessToken: accessToken, body: map[string]any{
		"name": name, "role": app.roleToken(roleName),
	}})
	expectStatus(app.t, resp, http.StatusCreated)
	var body botBody
	resp.decode(app.t, &body)
	return body
}

func (app *testApp) botToken(token, secret string) response {
	app.t.Helper()
	return app.do(request{
		method: "POST", path: "/api/core/v1/oauth/token", form: url.Values{"grant_type": {"client_credentials"}},
		basicUser: token, basicPassword: secret,
	})
}

func TestCreateBotReturnsSecretOnceAndReadsNeverReturnIt(t *testing.T) {
	app := newTestApp(t)
	admin := app.signedInAdmin()

	bot := app.createBot(admin.AccessToken, "Dev Bot", "member")

	if bot.Token == "" || bot.ClientSecret == "" || bot.Kind != "bot" || bot.Name != "Dev Bot" || bot.Role.Name != "member" || bot.Email != "" {
		t.Fatalf("created bot = %+v", bot)
	}
	list := app.do(request{method: "GET", path: "/api/core/v1/members", accessToken: admin.AccessToken})
	expectStatus(t, list, http.StatusOK)
	if strings.Contains(string(list.body), bot.ClientSecret) || !strings.Contains(string(list.body), `"kind":"bot"`) {
		t.Errorf("member list = %s", list.body)
	}
}

func TestBotManagementNeedsMemberManage(t *testing.T) {
	app := newTestApp(t)
	app.createUser("bruno@example.com", "Bruno", "member")
	member := app.loginNative("bruno@example.com")

	resp := app.do(request{method: "POST", path: "/api/core/v1/bots", accessToken: member.AccessToken, body: map[string]any{
		"name": "Dev Bot", "role": app.roleToken("member"),
	}})

	expectError(t, resp, http.StatusForbidden, "forbidden")
	for _, call := range []request{
		{method: "PATCH", path: "/api/core/v1/bots/x", body: map[string]any{"name": "N"}},
		{method: "POST", path: "/api/core/v1/bots/x/secret"},
		{method: "DELETE", path: "/api/core/v1/bots/x"},
	} {
		call.accessToken = member.AccessToken
		expectError(t, app.do(call), http.StatusForbidden, "forbidden")
	}
}

func TestCreateBotValidatesInput(t *testing.T) {
	app := newTestApp(t)
	admin := app.signedInAdmin()

	resp := app.do(request{method: "POST", path: "/api/core/v1/bots", accessToken: admin.AccessToken, body: map[string]any{"name": " ", "role": ""}})

	expectStatus(t, resp, http.StatusBadRequest)
}

func TestBotRenameRoleChangeAndDeactivate(t *testing.T) {
	app := newTestApp(t)
	admin := app.signedInAdmin()
	bot := app.createBot(admin.AccessToken, "Dev Bot", "member")

	renamed := app.do(request{method: "PATCH", path: "/api/core/v1/bots/" + bot.Token, accessToken: admin.AccessToken, body: map[string]any{"name": "Reviewer"}})
	expectStatus(t, renamed, http.StatusOK)
	var afterRename botBody
	renamed.decode(t, &afterRename)
	if afterRename.Name != "Reviewer" || afterRename.Role.Name != "member" || afterRename.Status != "active" {
		t.Errorf("single-field PATCH changed other fields: %+v", afterRename)
	}

	resp := app.do(request{method: "PATCH", path: "/api/core/v1/bots/" + bot.Token, accessToken: admin.AccessToken, body: map[string]any{"role": app.roleToken("admin"), "status": "deactivated"}})
	expectStatus(t, resp, http.StatusOK)
	var changed botBody
	resp.decode(t, &changed)
	if changed.Name != "Reviewer" || changed.Role.Name != "admin" || changed.Status != "deactivated" {
		t.Errorf("changed bot = %+v", changed)
	}
	expectStatus(t, app.do(request{method: "PATCH", path: "/api/core/v1/bots/" + bot.Token, accessToken: admin.AccessToken, body: map[string]any{"name": nil}}), http.StatusBadRequest)
}

func TestBotAPIDoesNotTouchHumans(t *testing.T) {
	app := newTestApp(t)
	admin := app.signedInAdmin()
	human := app.createUser("bruno@example.com", "Bruno", "member")

	resp := app.do(request{method: "POST", path: "/api/core/v1/bots/" + human.Token + "/secret", accessToken: admin.AccessToken})

	expectError(t, resp, http.StatusNotFound, "member_not_found")
}

func TestRemoveBot(t *testing.T) {
	app := newTestApp(t)
	admin := app.signedInAdmin()
	bot := app.createBot(admin.AccessToken, "Dev Bot", "member")

	expectStatus(t, app.do(request{method: "DELETE", path: "/api/core/v1/bots/" + bot.Token, accessToken: admin.AccessToken}), http.StatusNoContent)

	expectStatus(t, app.botToken(bot.Token, bot.ClientSecret), http.StatusUnauthorized)
	list := app.do(request{method: "GET", path: "/api/core/v1/members", accessToken: admin.AccessToken})
	if strings.Contains(string(list.body), bot.Token) {
		t.Errorf("removed bot is listed: %s", list.body)
	}
}

func TestClientCredentialsIssuesShortLivedTokenWithoutRefreshToken(t *testing.T) {
	app := newTestApp(t)
	admin := app.signedInAdmin()
	bot := app.createBot(admin.AccessToken, "Dev Bot", "member")

	resp := app.botToken(bot.Token, bot.ClientSecret)

	expectStatus(t, resp, http.StatusOK)
	var token tokenBody
	resp.decode(t, &token)
	if token.AccessToken == "" || token.RefreshToken != "" || token.ExpiresIn <= 0 || token.ExpiresIn > 15*60 || token.TokenType != "Bearer" {
		t.Fatalf("token = %+v", token)
	}
	if strings.Contains(string(resp.body), "refresh_token") || resp.header.Get("Cache-Control") != "no-store" {
		t.Errorf("response = %s, %v", resp.body, resp.header)
	}
	me := app.do(request{method: "GET", path: "/api/core/v1/me", accessToken: token.AccessToken})
	expectStatus(t, me, http.StatusOK)
	var meResponse struct {
		Token       string                `json:"token"`
		Kind        string                `json:"kind"`
		Role        struct{ Name string } `json:"role"`
		Permissions []string              `json:"permissions"`
	}
	me.decode(t, &meResponse)
	if meResponse.Token != bot.Token || meResponse.Kind != "bot" || meResponse.Role.Name != "member" || len(meResponse.Permissions) == 0 {
		t.Errorf("me = %+v", meResponse)
	}
}

func TestBotAccessTokenStopsWorkingAfterItsLifetime(t *testing.T) {
	app := newTestApp(t)
	admin := app.signedInAdmin()
	bot := app.createBot(admin.AccessToken, "Dev Bot", "member")
	var token tokenBody
	app.botToken(bot.Token, bot.ClientSecret).decode(t, &token)

	app.clock.Advance(16 * time.Minute)

	expectStatus(t, app.do(request{method: "GET", path: "/api/core/v1/me", accessToken: token.AccessToken}), http.StatusUnauthorized)
}

func TestClientCredentialsRefusalsAreIndistinguishable(t *testing.T) {
	app := newTestApp(t)
	admin := app.signedInAdmin()
	bot := app.createBot(admin.AccessToken, "Dev Bot", "member")
	deactivated := app.createBot(admin.AccessToken, "Old Bot", "member")
	expectStatus(t, app.do(request{method: "PATCH", path: "/api/core/v1/bots/" + deactivated.Token, accessToken: admin.AccessToken, body: map[string]any{"status": "deactivated"}}), http.StatusOK)

	wrongSecret := app.botToken(bot.Token, "wrong")
	deactivatedBot := app.botToken(deactivated.Token, deactivated.ClientSecret)
	unknownBot := app.botToken("unknown", "wrong")

	for _, resp := range []response{wrongSecret, deactivatedBot, unknownBot} {
		expectStatus(t, resp, http.StatusUnauthorized)
		var body oauthErrorBody
		resp.decode(t, &body)
		if body.Error != "invalid_client" {
			t.Errorf("error = %+v", body)
		}
	}
	if string(wrongSecret.body) != string(deactivatedBot.body) || string(wrongSecret.body) != string(unknownBot.body) {
		t.Errorf("bodies differ: %s | %s | %s", wrongSecret.body, deactivatedBot.body, unknownBot.body)
	}
}

func TestClientCredentialsNeedsBasicCredentials(t *testing.T) {
	app := newTestApp(t)

	resp := app.do(request{method: "POST", path: "/api/core/v1/oauth/token", form: url.Values{"grant_type": {"client_credentials"}}})

	expectStatus(t, resp, http.StatusUnauthorized)
}

func TestRotatedSecretReplacesThePreviousOne(t *testing.T) {
	app := newTestApp(t)
	admin := app.signedInAdmin()
	bot := app.createBot(admin.AccessToken, "Dev Bot", "member")

	resp := app.do(request{method: "POST", path: "/api/core/v1/bots/" + bot.Token + "/secret", accessToken: admin.AccessToken})

	expectStatus(t, resp, http.StatusOK)
	var rotated botBody
	resp.decode(t, &rotated)
	if rotated.ClientSecret == "" || rotated.ClientSecret == bot.ClientSecret {
		t.Fatalf("rotated = %+v", rotated)
	}
	expectStatus(t, app.botToken(bot.Token, bot.ClientSecret), http.StatusUnauthorized)
	expectStatus(t, app.botToken(bot.Token, rotated.ClientSecret), http.StatusOK)
}

func TestClientCredentialsIsRateLimitedPerBot(t *testing.T) {
	limits := generousRateLimits
	limits.BotToken = platform.RateLimit{PerMinute: 1, Burst: 2}
	app := newTestApp(t, withRateLimits(limits))
	admin := app.signedInAdmin()
	bot := app.createBot(admin.AccessToken, "Dev Bot", "member")
	other := app.createBot(admin.AccessToken, "Other Bot", "member")

	statuses := []int{
		app.botToken(bot.Token, bot.ClientSecret).status, app.botToken(bot.Token, bot.ClientSecret).status, app.botToken(bot.Token, bot.ClientSecret).status,
	}

	if statuses[0] != http.StatusOK || statuses[1] != http.StatusOK || statuses[2] != http.StatusTooManyRequests {
		t.Errorf("statuses = %v", statuses)
	}
	expectStatus(t, app.botToken(other.Token, other.ClientSecret), http.StatusOK)
}

func TestPasswordLoginIsGenericForBots(t *testing.T) {
	app := newTestApp(t)
	admin := app.signedInAdmin()
	app.createBot(admin.AccessToken, "devbot@bots.example", "member")

	resp := app.do(request{method: "POST", path: "/api/core/v1/auth/login", body: map[string]any{
		"email": "devbot@bots.example", "password": "anything long enough", "client": "native",
	}})

	expectError(t, resp, http.StatusUnauthorized, "invalid_credentials")
}

func TestDirectoryAndMeReportKind(t *testing.T) {
	app := newTestApp(t)
	admin := app.signedInAdmin()
	bot := app.createBot(admin.AccessToken, "Dev Bot", "member")

	resp := app.do(request{method: "GET", path: "/api/core/v1/users", accessToken: admin.AccessToken})

	expectStatus(t, resp, http.StatusOK)
	var directory struct {
		Users []struct {
			Token string `json:"token"`
			Name  string `json:"name"`
			Kind  string `json:"kind"`
		} `json:"users"`
	}
	resp.decode(t, &directory)
	kinds := map[string]string{}
	for _, user := range directory.Users {
		kinds[user.Name] = user.Kind
	}
	if kinds["Dev Bot"] != "bot" || kinds["Ana"] != "human" {
		t.Errorf("directory kinds = %v", kinds)
	}
	_ = bot
}
