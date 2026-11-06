package app_test

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func (app *testApp) grantDelegatedSession(accessToken, module, purpose string) response {
	app.t.Helper()
	return app.do(request{method: "POST", path: "/api/core/v1/delegated-sessions", accessToken: accessToken, body: map[string]any{
		"module": module, "purpose": purpose,
	}})
}

func (app *testApp) refreshWithBody(refreshToken string) response {
	app.t.Helper()
	return app.do(request{method: "POST", path: "/api/core/v1/auth/refresh", body: map[string]any{"refreshToken": refreshToken}})
}

func (app *testApp) delegatedAdminSession() (signInBody, signInBody) {
	app.t.Helper()
	expectStatus(app.t, app.registerModule(gentasksManifest()), http.StatusOK)
	admin := app.signedInAdmin()
	resp := app.grantDelegatedSession(admin.AccessToken, "gentasks", "Admin chat")
	expectStatus(app.t, resp, http.StatusOK)
	var delegated signInBody
	resp.decode(app.t, &delegated)
	return admin, delegated
}

func TestGrantDelegatedSessionReturnsRefreshTokenOnce(t *testing.T) {
	app := newTestApp(t)

	admin, delegated := app.delegatedAdminSession()

	if delegated.RefreshToken == "" || delegated.AccessToken == "" || delegated.Session == admin.Session {
		t.Fatalf("delegated = %+v", delegated)
	}
	list := app.do(request{method: "GET", path: "/api/core/v1/auth/sessions", accessToken: admin.AccessToken})
	var body struct {
		Sessions []struct {
			Token      string  `json:"token"`
			ClientType string  `json:"clientType"`
			Module     *string `json:"module"`
			Purpose    *string `json:"purpose"`
		} `json:"sessions"`
	}
	list.decode(t, &body)
	found := false
	for _, session := range body.Sessions {
		if session.Token == delegated.Session {
			found = session.ClientType == "delegated" && session.Module != nil && *session.Module == "gentasks" && session.Purpose != nil && *session.Purpose == "Admin chat"
		}
	}
	if !found || strings.Contains(string(list.body), delegated.RefreshToken) {
		t.Errorf("sessions = %s", list.body)
	}
}

func TestGrantDelegatedSessionRules(t *testing.T) {
	app := newTestApp(t)
	expectStatus(t, app.registerModule(gentasksManifest()), http.StatusOK)
	admin := app.signedInAdmin()

	expectError(t, app.grantDelegatedSession("", "gentasks", "x"), http.StatusUnauthorized, "invalid_token")
	expectStatus(t, app.grantDelegatedSession(admin.AccessToken, "ghost", "x"), http.StatusBadRequest)
	expectStatus(t, app.grantDelegatedSession(admin.AccessToken, "gentasks", " "), http.StatusBadRequest)
	expectStatus(t, app.grantDelegatedSession(admin.AccessToken, "", "x"), http.StatusBadRequest)
	expectStatus(t, app.grantDelegatedSession(admin.AccessToken, "gentasks", strings.Repeat("x", 101)), http.StatusBadRequest)
	if _, err := app.db.Exec(t.Context(), `UPDATE modules SET enabled = false WHERE name = 'gentasks'`); err != nil {
		t.Fatalf("disable module: %v", err)
	}
	expectStatus(t, app.grantDelegatedSession(admin.AccessToken, "gentasks", "Admin chat"), http.StatusBadRequest)
}

func TestDelegatedRefreshRotatesAndReuseRevokesTheSession(t *testing.T) {
	app := newTestApp(t)
	_, delegated := app.delegatedAdminSession()

	first := app.refreshWithBody(delegated.RefreshToken)
	expectStatus(t, first, http.StatusOK)
	var rotated signInBody
	first.decode(t, &rotated)
	if rotated.RefreshToken == "" || rotated.RefreshToken == delegated.RefreshToken || rotated.Session != delegated.Session {
		t.Fatalf("rotated = %+v", rotated)
	}

	expectStatus(t, app.refreshWithBody(delegated.RefreshToken), http.StatusUnauthorized)
	expectStatus(t, app.refreshWithBody(rotated.RefreshToken), http.StatusUnauthorized)
}

func TestUserEndsDelegatedSession(t *testing.T) {
	app := newTestApp(t)
	admin, delegated := app.delegatedAdminSession()

	end := app.do(request{method: "DELETE", path: "/api/core/v1/auth/sessions/" + delegated.Session, accessToken: admin.AccessToken})

	expectStatus(t, end, http.StatusNoContent)
	expectStatus(t, app.refreshWithBody(delegated.RefreshToken), http.StatusUnauthorized)
}

func TestDeactivatingTheMemberEndsDelegatedSession(t *testing.T) {
	app := newTestApp(t)
	expectStatus(t, app.registerModule(gentasksManifest()), http.StatusOK)
	admin := app.signedInAdmin()
	app.createUser("bruno@example.com", "Bruno", "member")
	bruno := app.loginNative("bruno@example.com")
	var delegated signInBody
	app.grantDelegatedSession(bruno.AccessToken, "gentasks", "Chat").decode(t, &delegated)
	brunoToken := app.storedUserByEmail("bruno@example.com").Token

	resp := app.do(request{method: "PATCH", path: "/api/core/v1/members/" + brunoToken, accessToken: admin.AccessToken, body: map[string]any{"status": "deactivated"}})

	expectStatus(t, resp, http.StatusOK)
	expectStatus(t, app.refreshWithBody(delegated.RefreshToken), http.StatusUnauthorized)
}

func TestDelegatedPermissionsAreRecomputedOnRefresh(t *testing.T) {
	app := newTestApp(t)
	expectStatus(t, app.registerModule(gentasksManifest()), http.StatusOK)
	admin := app.signedInAdmin()
	app.createUser("bruno@example.com", "Bruno", "admin")
	bruno := app.loginNative("bruno@example.com")
	var delegated signInBody
	app.grantDelegatedSession(bruno.AccessToken, "gentasks", "Chat").decode(t, &delegated)
	if !contains(app.meBody(delegated.AccessToken).Permissions, "core.member.manage") {
		t.Fatal("delegated session of an admin lacks admin permissions")
	}
	brunoToken := app.storedUserByEmail("bruno@example.com").Token
	expectStatus(t, app.do(request{method: "PATCH", path: "/api/core/v1/members/" + brunoToken, accessToken: admin.AccessToken, body: map[string]any{"role": app.roleToken("member")}}), http.StatusOK)

	var refreshed signInBody
	app.refreshWithBody(delegated.RefreshToken).decode(t, &refreshed)

	if contains(app.meBody(refreshed.AccessToken).Permissions, "core.member.manage") {
		t.Error("permissions were not recomputed after the role change")
	}
}

func TestDelegatedSessionIdleAndAbsoluteLimits(t *testing.T) {
	app := newTestApp(t)
	_, delegated := app.delegatedAdminSession()

	app.clock.Advance(25 * time.Hour)

	expectError(t, app.refreshWithBody(delegated.RefreshToken), http.StatusUnauthorized, "session_expired")

	_, active := app.delegatedAdminSessionFor("ana2@example.com")
	current := active
	for step := 0; step < 8; step++ {
		app.clock.Advance(20 * time.Hour)
		resp := app.refreshWithBody(current.RefreshToken)
		expectStatus(t, resp, http.StatusOK)
		resp.decode(t, &current)
	}
	app.clock.Advance(20 * time.Hour)
	expectError(t, app.refreshWithBody(current.RefreshToken), http.StatusUnauthorized, "session_expired")
}

func (app *testApp) delegatedAdminSessionFor(email string) (signInBody, signInBody) {
	app.t.Helper()
	app.createUser(email, "Second", "admin")
	admin := app.loginNative(email)
	resp := app.grantDelegatedSession(admin.AccessToken, "gentasks", "Admin chat")
	expectStatus(app.t, resp, http.StatusOK)
	var delegated signInBody
	resp.decode(app.t, &delegated)
	return admin, delegated
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func TestOnlyAPersonsOwnSignInCanGrantADelegatedSession(t *testing.T) {
	app := newTestApp(t)
	admin, delegated := app.delegatedAdminSession()
	bot := app.createBot(admin.AccessToken, "Dev Bot", "member")
	var botSignIn struct {
		AccessToken string `json:"access_token"`
	}
	app.botToken(bot.Token, bot.ClientSecret).decode(t, &botSignIn)

	expectError(t, app.grantDelegatedSession(botSignIn.AccessToken, "gentasks", "Chat"), http.StatusForbidden, "forbidden")
	expectError(t, app.grantDelegatedSession(delegated.AccessToken, "gentasks", "Chat"), http.StatusForbidden, "forbidden")
	expectStatus(t, app.grantDelegatedSession(admin.AccessToken, "gentasks", "Chat"), http.StatusOK)
}
