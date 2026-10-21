package app_test

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/access"
	"github.com/JoaoBittencourt1/Genwork/core/internal/auth"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/core/internal/users"
)

func TestDiscoveryDocument(t *testing.T) {
	app := newTestApp(t)
	resp := app.do(request{method: "GET", path: "/.well-known/genwork"})

	expectStatus(t, resp, http.StatusOK)
	if resp.header.Get("Cache-Control") != "public, max-age=300" || resp.header.Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("headers = %v", resp.header)
	}
	var document struct {
		Product string `json:"product"`
		Name    string `json:"name"`
		Version string `json:"version"`
		API     string `json:"api"`
		Auth    struct {
			Methods []string `json:"methods"`
		} `json:"auth"`
	}
	resp.decode(t, &document)
	if document.Product != "genwork" || document.Name != testInstanceName || document.Version != "test" || document.API != "v1" ||
		len(document.Auth.Methods) != 1 || document.Auth.Methods[0] != "password" {
		t.Errorf("document = %+v", document)
	}
}

func TestMigrationsSeedRolesAndPermissions(t *testing.T) {
	app := newTestApp(t)
	roles, err := access.RoleRepository{}.List(context.Background(), app.db)
	if err != nil {
		t.Fatalf("list roles: %v", err)
	}
	byName := map[string]access.Role{}
	for _, role := range roles {
		byName[role.Name] = role
	}
	admin, member := byName["admin"], byName["member"]
	if len(admin.Permissions) != 4 || !admin.Grants(access.PermissionUserRead) || !admin.Grants(access.PermissionInviteCreate) || !admin.Grants(access.PermissionInviteRead) || !admin.Grants(access.PermissionInviteRevoke) {
		t.Errorf("admin permissions = %v", admin.Permissions)
	}
	if len(member.Permissions) != 1 || !member.Grants(access.PermissionUserRead) {
		t.Errorf("member permissions = %v", member.Permissions)
	}
	if len(admin.Token) != 22 {
		t.Errorf("public token %q should be 22 base64url characters", admin.Token)
	}
}

func TestUserEmailIsUniqueAmongActiveUsers(t *testing.T) {
	app := newTestApp(t)
	first := app.createUser("ana@example.com", "Ana", "admin")

	_, err := users.NewService(users.NewPasswordHasher(testPasswordParams, 1)).Create(context.Background(), app.db, users.CreateInput{
		Email: " ANA@example.com ", Name: "Other", Password: testPassword, RoleID: first.RoleID,
	})
	if err != users.ErrEmailAlreadyRegistered {
		t.Fatalf("expected email_already_registered, got %v", err)
	}
	if _, err := app.db.Exec(context.Background(), `UPDATE users SET deleted_at = now() WHERE id = $1`, first.ID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	app.createUser("ana@example.com", "Ana again", "member")
}

func TestNativeLoginReturnsRefreshTokenInBody(t *testing.T) {
	app := newTestApp(t)
	user := app.createUser("ana@example.com", "Ana", "admin")

	signIn := app.loginNative(" Ana@Example.com ")
	if signIn.AccessToken == "" || signIn.RefreshToken == "" || signIn.Session == "" {
		t.Fatalf("incomplete sign-in: %+v", signIn)
	}
	if !signIn.AccessTokenExpiresAt.Equal(app.clock.Now().Add(15 * time.Minute)) {
		t.Errorf("access token expiry = %s", signIn.AccessTokenExpiresAt)
	}
	stored, _ := users.Repository{}.FindByID(context.Background(), app.db, user.ID)
	if stored.LastLoginAt == nil || !stored.LastLoginAt.Equal(app.clock.Now()) {
		t.Errorf("last_login_at = %v", stored.LastLoginAt)
	}
}

func TestWebLoginSetsHardenedRefreshCookie(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "admin")

	resp := app.do(request{method: "POST", path: "/api/core/v1/auth/login", body: map[string]any{
		"email": "ana@example.com", "password": testPassword, "client": "web",
	}})
	expectStatus(t, resp, http.StatusOK)
	if strings.Contains(string(resp.body), "refreshToken") {
		t.Errorf("web response must not carry the refresh token: %s", resp.body)
	}
	cookie := resp.cookie(auth.RefreshCookieName)
	if cookie == nil || cookie.Value == "" {
		t.Fatal("missing refresh cookie")
	}
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/api/core/v1/auth" {
		t.Errorf("cookie attributes = %+v", cookie)
	}
}

func TestLoginErrorsAreUniform(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "admin")

	unknown := app.do(request{method: "POST", path: "/api/core/v1/auth/login", body: map[string]any{
		"email": "nobody@example.com", "password": testPassword, "client": "native",
	}})
	wrong := app.do(request{method: "POST", path: "/api/core/v1/auth/login", body: map[string]any{
		"email": "ana@example.com", "password": "wrong password!", "client": "native",
	}})
	expectError(t, unknown, http.StatusUnauthorized, "invalid_credentials")
	expectError(t, wrong, http.StatusUnauthorized, "invalid_credentials")
	if string(unknown.body) != string(wrong.body) {
		t.Errorf("bodies differ: %s vs %s", unknown.body, wrong.body)
	}
}

func TestLoginValidatesInput(t *testing.T) {
	app := newTestApp(t)
	resp := app.do(request{method: "POST", path: "/api/core/v1/auth/login", body: map[string]any{
		"email": "not-an-email", "password": testPassword, "client": "desktop",
	}})
	expectError(t, resp, http.StatusBadRequest, "validation_failed")
	var body struct {
		Error struct {
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	resp.decode(t, &body)
	if body.Error.Fields["email"] != "invalid" || body.Error.Fields["client"] != "invalid" {
		t.Errorf("fields = %v", body.Error.Fields)
	}
}

func TestAccountLocksAfterTenFailures(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "admin")
	attempt := func(password string) response {
		return app.do(request{method: "POST", path: "/api/core/v1/auth/login", body: map[string]any{
			"email": "ana@example.com", "password": password, "client": "native",
		}})
	}
	for failure := 0; failure < auth.LockoutThreshold; failure++ {
		expectError(t, attempt("wrong password!"), http.StatusUnauthorized, "invalid_credentials")
	}
	expectError(t, attempt(testPassword), http.StatusUnauthorized, "invalid_credentials")

	app.clock.Advance(auth.LockoutDuration)
	expectStatus(t, attempt(testPassword), http.StatusOK)
}

func TestLoginUpgradesOutdatedPasswordHash(t *testing.T) {
	stronger := testPasswordParams
	stronger.MemoryKiB = 16 * 1024
	app := newTestApp(t, withPasswordParams(stronger))
	user := app.createUserWithParams("ana@example.com", "Ana", "admin", testPasswordParams)

	app.loginNative("ana@example.com")

	stored, _ := users.Repository{}.FindByID(context.Background(), app.db, user.ID)
	if !strings.Contains(stored.PasswordHash, "m=16384") {
		t.Errorf("hash was not upgraded: %s", stored.PasswordHash)
	}
	app.loginNative("ana@example.com")
}

func TestLoginIsRateLimitedPerAddress(t *testing.T) {
	limits := generousRateLimits
	limits.Login = platform.RateLimit{PerMinute: 2, Burst: 2}
	app := newTestApp(t, withRateLimits(limits))
	body := map[string]any{"email": "nobody@example.com", "password": testPassword, "client": "native"}

	app.do(request{method: "POST", path: "/api/core/v1/auth/login", body: body})
	app.do(request{method: "POST", path: "/api/core/v1/auth/login", body: body})
	resp := app.do(request{method: "POST", path: "/api/core/v1/auth/login", body: body})

	expectError(t, resp, http.StatusTooManyRequests, "rate_limited")
	if resp.header.Get("Retry-After") == "" {
		t.Error("missing Retry-After")
	}
}

func TestMeReturnsProfileWithoutInternals(t *testing.T) {
	app := newTestApp(t)
	user := app.createUser("ana@example.com", "Ana", "admin")
	signIn := app.loginNative("ana@example.com")

	resp := app.do(request{method: "GET", path: "/api/core/v1/me", accessToken: signIn.AccessToken})
	expectStatus(t, resp, http.StatusOK)
	var me struct {
		Token       string   `json:"token"`
		Email       string   `json:"email"`
		Name        string   `json:"name"`
		Permissions []string `json:"permissions"`
		Role        struct {
			Name string `json:"name"`
		} `json:"role"`
	}
	resp.decode(t, &me)
	if me.Token != user.Token || me.Email != "ana@example.com" || me.Name != "Ana" || me.Role.Name != "admin" || len(me.Permissions) != 4 {
		t.Errorf("me = %+v", me)
	}
	for _, leaked := range []string{"password", "argon2", `"id"`} {
		if strings.Contains(string(resp.body), leaked) {
			t.Errorf("response leaks %s: %s", leaked, resp.body)
		}
	}
}

func TestProtectedEndpointsRejectBadTokens(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "admin")
	signIn := app.loginNative("ana@example.com")

	expectError(t, app.do(request{method: "GET", path: "/api/core/v1/me"}), http.StatusUnauthorized, "invalid_token")
	expectError(t, app.do(request{method: "GET", path: "/api/core/v1/me", accessToken: "garbage"}), http.StatusUnauthorized, "invalid_token")

	app.clock.Advance(16 * time.Minute)
	expectError(t, app.do(request{method: "GET", path: "/api/core/v1/me", accessToken: signIn.AccessToken}), http.StatusUnauthorized, "invalid_token")
}

func TestJWKSIsPublic(t *testing.T) {
	app := newTestApp(t)
	resp := app.do(request{method: "GET", path: "/api/core/v1/auth/jwks"})
	expectStatus(t, resp, http.StatusOK)
	var set auth.JWKSet
	resp.decode(t, &set)
	if len(set.Keys) != 1 || set.Keys[0].Curve != "Ed25519" {
		t.Errorf("jwks = %+v", set)
	}
}

func TestNativeRefreshRotatesAndDetectsReuse(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "admin")
	signIn := app.loginNative("ana@example.com")

	first := app.do(request{method: "POST", path: "/api/core/v1/auth/refresh", body: map[string]any{"refreshToken": signIn.RefreshToken}})
	expectStatus(t, first, http.StatusOK)
	var rotated signInBody
	first.decode(t, &rotated)
	if rotated.RefreshToken == "" || rotated.RefreshToken == signIn.RefreshToken || rotated.Session != signIn.Session {
		t.Fatalf("unexpected rotation: %+v", rotated)
	}

	reuse := app.do(request{method: "POST", path: "/api/core/v1/auth/refresh", body: map[string]any{"refreshToken": signIn.RefreshToken}})
	expectError(t, reuse, http.StatusUnauthorized, "session_revoked")

	afterTheft := app.do(request{method: "POST", path: "/api/core/v1/auth/refresh", body: map[string]any{"refreshToken": rotated.RefreshToken}})
	expectError(t, afterTheft, http.StatusUnauthorized, "session_revoked")
	expectError(t, app.do(request{method: "GET", path: "/api/core/v1/me", accessToken: rotated.AccessToken}), http.StatusUnauthorized, "invalid_token")
}

func TestUnknownRefreshTokenIsRejected(t *testing.T) {
	app := newTestApp(t)
	resp := app.do(request{method: "POST", path: "/api/core/v1/auth/refresh", body: map[string]any{"refreshToken": "nope"}})
	expectError(t, resp, http.StatusUnauthorized, "invalid_token")
}

func TestWebRefreshUsesCookieAndClientHeader(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "admin")
	login := app.do(request{method: "POST", path: "/api/core/v1/auth/login", body: map[string]any{
		"email": "ana@example.com", "password": testPassword, "client": "web",
	}})
	cookie := login.cookie(auth.RefreshCookieName)

	withoutHeader := app.do(request{method: "POST", path: "/api/core/v1/auth/refresh", cookies: []*http.Cookie{cookie}})
	if withoutHeader.status == http.StatusOK {
		t.Fatal("cookie alone must not refresh the session")
	}

	refreshed := app.do(request{method: "POST", path: "/api/core/v1/auth/refresh", webClient: true, cookies: []*http.Cookie{cookie}})
	expectStatus(t, refreshed, http.StatusOK)
	newCookie := refreshed.cookie(auth.RefreshCookieName)
	if newCookie == nil || newCookie.Value == cookie.Value {
		t.Fatal("refresh must rotate the cookie")
	}
	if strings.Contains(string(refreshed.body), "refreshToken") {
		t.Error("web refresh must not return the token in the body")
	}
}

func TestSessionExpiresWhenIdle(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "admin")
	signIn := app.loginNative("ana@example.com")

	app.clock.Advance(31 * 24 * time.Hour)
	resp := app.do(request{method: "POST", path: "/api/core/v1/auth/refresh", body: map[string]any{"refreshToken": signIn.RefreshToken}})
	expectError(t, resp, http.StatusUnauthorized, "session_expired")
}

func TestSessionHasAbsoluteLifetime(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "admin")
	refreshToken := app.loginNative("ana@example.com").RefreshToken

	for elapsedDays := 20; elapsedDays < 90; elapsedDays += 20 {
		app.clock.Advance(20 * 24 * time.Hour)
		resp := app.do(request{method: "POST", path: "/api/core/v1/auth/refresh", body: map[string]any{"refreshToken": refreshToken}})
		expectStatus(t, resp, http.StatusOK)
		var rotated signInBody
		resp.decode(t, &rotated)
		refreshToken = rotated.RefreshToken
	}
	app.clock.Advance(11 * 24 * time.Hour)
	resp := app.do(request{method: "POST", path: "/api/core/v1/auth/refresh", body: map[string]any{"refreshToken": refreshToken}})
	expectError(t, resp, http.StatusUnauthorized, "session_expired")
}

func TestWebLogoutRevokesSessionAndClearsCookie(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "admin")
	login := app.do(request{method: "POST", path: "/api/core/v1/auth/login", body: map[string]any{
		"email": "ana@example.com", "password": testPassword, "client": "web",
	}})
	var signIn signInBody
	login.decode(t, &signIn)

	logout := app.do(request{method: "POST", path: "/api/core/v1/auth/logout", accessToken: signIn.AccessToken, webClient: true})
	expectStatus(t, logout, http.StatusNoContent)
	if cleared := logout.cookie(auth.RefreshCookieName); cleared == nil || cleared.MaxAge >= 0 {
		t.Errorf("cookie not cleared: %+v", cleared)
	}
	expectError(t, app.do(request{method: "GET", path: "/api/core/v1/me", accessToken: signIn.AccessToken}), http.StatusUnauthorized, "invalid_token")
	refresh := app.do(request{method: "POST", path: "/api/core/v1/auth/refresh", webClient: true, cookies: []*http.Cookie{login.cookie(auth.RefreshCookieName)}})
	expectError(t, refresh, http.StatusUnauthorized, "session_revoked")
}

func TestListAndRevokeOwnSessions(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "admin")
	app.createUser("bruno@example.com", "Bruno", "member")
	laptop := app.loginNative("ana@example.com")
	phone := app.loginNative("ana@example.com")
	bruno := app.loginNative("bruno@example.com")

	list := app.do(request{method: "GET", path: "/api/core/v1/auth/sessions", accessToken: laptop.AccessToken})
	expectStatus(t, list, http.StatusOK)
	var body struct {
		Sessions []struct {
			Token   string `json:"token"`
			Current bool   `json:"current"`
		} `json:"sessions"`
	}
	list.decode(t, &body)
	if len(body.Sessions) != 2 {
		t.Fatalf("sessions = %+v", body.Sessions)
	}
	for _, session := range body.Sessions {
		if session.Current != (session.Token == laptop.Session) {
			t.Errorf("wrong current flag: %+v", session)
		}
	}

	expectStatus(t, app.do(request{method: "DELETE", path: "/api/core/v1/auth/sessions/" + phone.Session, accessToken: laptop.AccessToken}), http.StatusNoContent)
	expectError(t, app.do(request{method: "GET", path: "/api/core/v1/me", accessToken: phone.AccessToken}), http.StatusUnauthorized, "invalid_token")

	others := app.do(request{method: "DELETE", path: "/api/core/v1/auth/sessions/" + bruno.Session, accessToken: laptop.AccessToken})
	expectError(t, others, http.StatusNotFound, "not_found")
	expectStatus(t, app.do(request{method: "GET", path: "/api/core/v1/me", accessToken: bruno.AccessToken}), http.StatusOK)
}

func TestConcurrentRefreshOfSameTokenSucceedsOnce(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "admin")
	signIn := app.loginNative("ana@example.com")

	var waitGroup sync.WaitGroup
	statuses := make(chan int, 2)
	for range 2 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			statuses <- app.do(request{method: "POST", path: "/api/core/v1/auth/refresh", body: map[string]any{"refreshToken": signIn.RefreshToken}}).status
		}()
	}
	waitGroup.Wait()
	close(statuses)
	succeeded := 0
	for status := range statuses {
		if status == http.StatusOK {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Errorf("successful refreshes = %d, want 1", succeeded)
	}
}
