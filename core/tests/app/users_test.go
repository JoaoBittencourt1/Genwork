package app_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
)

type directoryUserBody struct {
	Token string `json:"token"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func accessTokenPermissions(t *testing.T, accessToken string) []string {
	t.Helper()
	parts := strings.Split(accessToken, ".")
	if len(parts) != 3 {
		t.Fatalf("malformed access token %q", accessToken)
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	var claims struct {
		Permissions []string `json:"permissions"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatalf("decode claims: %v", err)
	}
	return claims.Permissions
}

func TestUserDirectoryListsActiveUsersByName(t *testing.T) {
	app := newTestApp(t)
	app.createUser("zoe@example.com", "Zoe", "admin")
	bruno := app.createUser("bruno@example.com", "Bruno", "member")
	gone := app.createUser("gone@example.com", "Anna Gone", "member")
	if _, err := app.db.Exec(context.Background(), `UPDATE users SET deleted_at = now() WHERE id = $1`, gone.ID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	signIn := app.loginNative("bruno@example.com")

	resp := app.do(request{method: "GET", path: "/api/core/v1/users", accessToken: signIn.AccessToken})

	expectStatus(t, resp, http.StatusOK)
	var body struct {
		Users []directoryUserBody `json:"users"`
	}
	resp.decode(t, &body)
	if len(body.Users) != 2 || body.Users[0].Name != "Bruno" || body.Users[1].Name != "Zoe" {
		t.Fatalf("users = %+v", body.Users)
	}
	if body.Users[0].Token != bruno.Token || body.Users[0].Email != "bruno@example.com" {
		t.Errorf("first user = %+v", body.Users[0])
	}
	for _, leaked := range []string{"password", "argon2", "locked", "role", `"id"`} {
		if strings.Contains(string(resp.body), leaked) {
			t.Errorf("response leaks %s: %s", leaked, resp.body)
		}
	}
}

func TestUserDirectoryFindsUserByToken(t *testing.T) {
	app := newTestApp(t)
	ana := app.createUser("ana@example.com", "Ana", "member")
	signIn := app.loginNative("ana@example.com")

	resp := app.do(request{method: "GET", path: "/api/core/v1/users/" + ana.Token, accessToken: signIn.AccessToken})

	expectStatus(t, resp, http.StatusOK)
	var body directoryUserBody
	resp.decode(t, &body)
	if body != (directoryUserBody{Token: ana.Token, Name: "Ana", Email: "ana@example.com"}) {
		t.Errorf("user = %+v", body)
	}
}

func TestUserDirectoryHidesUnknownAndDeletedUsers(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "member")
	gone := app.createUser("gone@example.com", "Gone", "member")
	if _, err := app.db.Exec(context.Background(), `UPDATE users SET deleted_at = now() WHERE id = $1`, gone.ID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	signIn := app.loginNative("ana@example.com")

	for _, token := range []string{"unknown-token", gone.Token} {
		resp := app.do(request{method: "GET", path: "/api/core/v1/users/" + token, accessToken: signIn.AccessToken})
		expectError(t, resp, http.StatusNotFound, "user_not_found")
	}
}

func TestUserDirectoryRequiresPermission(t *testing.T) {
	app := newTestApp(t)
	app.createRole("guest")
	app.createUser("guest@example.com", "Guest", "guest")
	signIn := app.loginNative("guest@example.com")

	expectError(t, app.do(request{method: "GET", path: "/api/core/v1/users", accessToken: signIn.AccessToken}), http.StatusForbidden, "forbidden")
	expectError(t, app.do(request{method: "GET", path: "/api/core/v1/users/x", accessToken: signIn.AccessToken}), http.StatusForbidden, "forbidden")
}

func TestAccessTokenCarriesRolePermissions(t *testing.T) {
	app := newTestApp(t)
	expectStatus(t, app.registerModule(gentasksManifest()), http.StatusOK)
	app.createUser("ana@example.com", "Ana", "member")

	permissions := accessTokenPermissions(t, app.loginNative("ana@example.com").AccessToken)

	if !slices.Equal(permissions, []string{"core.user.read", "gentasks.board.read"}) {
		t.Errorf("permissions claim = %v", permissions)
	}
}

func TestRefreshedAccessTokenDropsRemovedGrant(t *testing.T) {
	app := newTestApp(t)
	expectStatus(t, app.registerModule(gentasksManifest()), http.StatusOK)
	app.createUser("ana@example.com", "Ana", "member")
	signIn := app.loginNative("ana@example.com")
	_, err := app.db.Exec(context.Background(), `
		DELETE FROM role_permissions
		USING roles, permissions
		WHERE role_permissions.role_id = roles.id AND role_permissions.permission_id = permissions.id
		  AND roles.name = 'member' AND permissions.name = 'gentasks.board.read'`)
	if err != nil {
		t.Fatalf("revoke grant: %v", err)
	}

	resp := app.do(request{method: "POST", path: "/api/core/v1/auth/refresh", body: map[string]any{"refreshToken": signIn.RefreshToken}})

	expectStatus(t, resp, http.StatusOK)
	var refreshed signInBody
	resp.decode(t, &refreshed)
	if permissions := accessTokenPermissions(t, refreshed.AccessToken); !slices.Equal(permissions, []string{"core.user.read"}) {
		t.Errorf("permissions claim after refresh = %v", permissions)
	}
}
