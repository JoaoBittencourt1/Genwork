package app_test

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/core/internal/users"
)

func TestRestrictedModulePermissionsLeaveTokensAndMe(t *testing.T) {
	app := newTestApp(t)
	expectStatus(t, app.registerModule(gentasksManifest()), http.StatusOK)
	ana := app.createUser("ana@example.com", "Ana", "member")
	app.restrictModules(ana, "gentasks")

	signIn := app.loginNative("ana@example.com")

	if permissions := accessTokenPermissions(t, signIn.AccessToken); !slices.Equal(permissions, []string{"core.user.read"}) {
		t.Errorf("restricted token permissions = %v", permissions)
	}
	if me := app.meBody(signIn.AccessToken); !slices.Equal(me.Permissions, []string{"core.user.read"}) {
		t.Errorf("restricted /me permissions = %v", me.Permissions)
	}

	app.restrictModules(ana)
	resp := app.do(request{method: "POST", path: "/api/core/v1/auth/refresh", body: map[string]any{"refreshToken": signIn.RefreshToken}})

	expectStatus(t, resp, http.StatusOK)
	var refreshed signInBody
	resp.decode(t, &refreshed)
	if permissions := accessTokenPermissions(t, refreshed.AccessToken); !slices.Equal(permissions, []string{"core.user.read", "gentasks.board.read"}) {
		t.Errorf("permissions after lifting the restriction = %v", permissions)
	}
}

func TestCapabilitiesOmitModulesRestrictedForTheCaller(t *testing.T) {
	app := newTestApp(t)
	expectStatus(t, app.registerModule(gentasksManifest()), http.StatusOK)
	ana := app.createUser("ana@example.com", "Ana", "member")
	app.createUser("bruno@example.com", "Bruno", "member")
	app.restrictModules(ana, "gentasks")

	for email, wantModules := range map[string]int{"ana@example.com": 0, "bruno@example.com": 1} {
		resp := app.do(request{method: "GET", path: "/api/core/v1/capabilities", accessToken: app.loginNative(email).AccessToken})
		expectStatus(t, resp, http.StatusOK)
		var body capabilitiesBody
		resp.decode(t, &body)
		if len(body.Modules) != wantModules {
			t.Errorf("%s capabilities modules = %+v, want %d", email, body.Modules, wantModules)
		}
	}
}

func TestDeactivatedMemberCannotSignIn(t *testing.T) {
	app := newTestApp(t)
	ana := app.createUser("ana@example.com", "Ana", "member")
	app.deactivateDirectly(ana)

	resp := app.do(request{method: "POST", path: "/api/core/v1/auth/login", body: map[string]any{
		"email": "ana@example.com", "password": testPassword, "client": "native",
	}})

	expectError(t, resp, http.StatusUnauthorized, "invalid_credentials")
	if sessions := app.countSessions(ana); sessions != 0 {
		t.Errorf("sessions = %d, want none", sessions)
	}
	stored, err := users.Repository{}.FindByID(context.Background(), app.db, ana.ID)
	if err != nil {
		t.Fatalf("find user: %v", err)
	}
	if stored.FailedLoginCount != 0 {
		t.Errorf("failed login count = %d, want 0 for a correct password", stored.FailedLoginCount)
	}
}

func TestAccessTokenOfDeactivatedMemberIsRejected(t *testing.T) {
	app := newTestApp(t)
	ana := app.createUser("ana@example.com", "Ana", "member")
	signIn := app.loginNative("ana@example.com")

	app.deactivateDirectly(ana)

	expectError(t, app.do(request{method: "GET", path: "/api/core/v1/me", accessToken: signIn.AccessToken}), http.StatusUnauthorized, "invalid_token")
}

func TestListRolesWithMemberManagementOnly(t *testing.T) {
	app := newTestApp(t)
	app.createRole("member-manager", "core.member.manage")
	app.createUser("ana@example.com", "Ana", "member-manager")

	resp := app.do(request{method: "GET", path: "/api/core/v1/roles", accessToken: app.loginNative("ana@example.com").AccessToken})

	expectStatus(t, resp, http.StatusOK)
}
