package app_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestListMembers(t *testing.T) {
	app := newTestApp(t)
	expectStatus(t, app.registerModule(gentasksManifest()), http.StatusOK)
	admin := app.signedInAdmin()
	bruno := app.createUser("bruno@example.com", "Bruno", "member")
	carla := app.createUser("carla@example.com", "Carla", "member")
	gone := app.createUser("dora@example.com", "Dora", "member")
	app.restrictModules(bruno, "gentasks")
	app.deactivateDirectly(carla)
	app.removeDirectly(gone)

	resp := app.do(request{method: "GET", path: "/api/core/v1/members", accessToken: admin.AccessToken})

	expectStatus(t, resp, http.StatusOK)
	var body struct {
		Members []memberBody `json:"members"`
	}
	resp.decode(t, &body)
	if len(body.Members) != 3 || body.Members[0].Name != "Ana" || body.Members[1].Name != "Bruno" || body.Members[2].Name != "Carla" {
		t.Fatalf("members = %+v", body.Members)
	}
	ana, brunoBody, carlaBody := body.Members[0], body.Members[1], body.Members[2]
	if ana.Role.Name != "admin" || ana.Role.Token != app.roleToken("admin") || ana.Status != "active" || ana.LastSignInAt == nil || ana.JoinedAt.IsZero() {
		t.Errorf("admin member = %+v", ana)
	}
	if brunoBody.Token != bruno.Token || brunoBody.Email != "bruno@example.com" || brunoBody.AvatarColor != string(bruno.AvatarColor()) ||
		brunoBody.LastSignInAt != nil || !slices.Equal(brunoBody.RestrictedModules, []string{"gentasks"}) {
		t.Errorf("restricted member = %+v", brunoBody)
	}
	if carlaBody.Status != "deactivated" || carlaBody.RestrictedModules == nil || len(carlaBody.RestrictedModules) != 0 {
		t.Errorf("deactivated member = %+v", carlaBody)
	}
	for _, leaked := range []string{"password", "argon2", "locked", "failed", `"id"`} {
		if strings.Contains(string(resp.body), leaked) {
			t.Errorf("response leaks %s: %s", leaked, resp.body)
		}
	}
}

func TestPatchMemberSingleFieldLeavesOtherFieldsUnchanged(t *testing.T) {
	app := newTestApp(t)
	expectStatus(t, app.registerModule(gentasksManifest()), http.StatusOK)
	admin := app.signedInAdmin()
	bruno := app.createUser("bruno@example.com", "Bruno", "admin")
	app.restrictModules(bruno, "gentasks")
	app.deactivateDirectly(bruno)

	steps := []struct {
		body       map[string]any
		role       string
		status     string
		restricted []string
	}{
		{body: map[string]any{"role": app.roleToken("member")}, role: "member", status: "deactivated", restricted: []string{"gentasks"}},
		{body: map[string]any{"status": "active"}, role: "member", status: "active", restricted: []string{"gentasks"}},
		{body: map[string]any{"restrictedModules": []string{}}, role: "member", status: "active", restricted: []string{}},
	}
	for _, step := range steps {
		resp := app.patchMember(admin.AccessToken, bruno.Token, step.body)
		expectStatus(t, resp, http.StatusOK)
		var patched memberBody
		resp.decode(t, &patched)
		stored := app.findMember(admin.AccessToken, bruno.Token)
		for _, member := range []memberBody{patched, stored} {
			if member.Role.Name != step.role || member.Status != step.status || !slices.Equal(member.RestrictedModules, step.restricted) ||
				member.Name != "Bruno" || member.Email != "bruno@example.com" {
				t.Errorf("after %v: member = %+v", step.body, member)
			}
		}
	}
}

func TestPatchMemberValidatesEveryField(t *testing.T) {
	app := newTestApp(t)
	admin := app.signedInAdmin()
	bruno := app.createUser("bruno@example.com", "Bruno", "member")

	invalidBodies := map[string]map[string]any{
		"role":              {"role": "unknown-role"},
		"status":            {"status": "banned"},
		"restrictedModules": {"restrictedModules": []string{"never-registered"}},
	}
	for field, body := range invalidBodies {
		resp := app.patchMember(admin.AccessToken, bruno.Token, body)
		expectError(t, resp, http.StatusBadRequest, "validation_failed")
		if !strings.Contains(string(resp.body), `"`+field+`"`) {
			t.Errorf("%v: error does not name %s: %s", body, field, resp.body)
		}
	}
	for _, field := range []string{"role", "status", "restrictedModules"} {
		resp := app.patchMember(admin.AccessToken, bruno.Token, map[string]any{field: nil})
		expectError(t, resp, http.StatusBadRequest, "validation_failed")
	}
	if member := app.findMember(admin.AccessToken, bruno.Token); member.Role.Name != "member" || member.Status != "active" || len(member.RestrictedModules) != 0 {
		t.Errorf("member changed by invalid requests: %+v", member)
	}
}

func TestDeactivateMemberRevokesEverySessionAndIsIdempotent(t *testing.T) {
	app := newTestApp(t)
	admin := app.signedInAdmin()
	bruno := app.createUser("bruno@example.com", "Bruno", "member")
	phone, laptop := app.loginNative("bruno@example.com"), app.loginNative("bruno@example.com")

	expectStatus(t, app.patchMember(admin.AccessToken, bruno.Token, map[string]any{"status": "deactivated"}), http.StatusOK)

	if sessions := app.countUnrevokedSessions(bruno); sessions != 0 {
		t.Errorf("unrevoked sessions = %d, want 0", sessions)
	}
	expectError(t, app.do(request{method: "GET", path: "/api/core/v1/me", accessToken: phone.AccessToken}), http.StatusUnauthorized, "invalid_token")
	refresh := app.do(request{method: "POST", path: "/api/core/v1/auth/refresh", body: map[string]any{"refreshToken": laptop.RefreshToken}})
	expectStatus(t, refresh, http.StatusUnauthorized)

	deactivatedAt := app.storedUser(bruno).DeactivatedAt
	app.clock.Advance(time.Minute)
	expectStatus(t, app.patchMember(admin.AccessToken, bruno.Token, map[string]any{"status": "deactivated"}), http.StatusOK)
	if again := app.storedUser(bruno).DeactivatedAt; deactivatedAt == nil || again == nil || !again.Equal(*deactivatedAt) {
		t.Errorf("deactivated at %v, then %v after deactivating again", deactivatedAt, again)
	}
}

func TestReactivatedMemberSignsInAgain(t *testing.T) {
	app := newTestApp(t)
	admin := app.signedInAdmin()
	bruno := app.createUser("bruno@example.com", "Bruno", "member")
	expectStatus(t, app.patchMember(admin.AccessToken, bruno.Token, map[string]any{"status": "deactivated"}), http.StatusOK)

	expectStatus(t, app.patchMember(admin.AccessToken, bruno.Token, map[string]any{"status": "active"}), http.StatusOK)

	app.loginNative("bruno@example.com")
}

func TestRemoveMember(t *testing.T) {
	app := newTestApp(t)
	admin := app.signedInAdmin()
	bruno := app.createUser("bruno@example.com", "Bruno", "member")
	brunoSignIn := app.loginNative("bruno@example.com")

	expectStatus(t, app.removeMember(admin.AccessToken, bruno.Token), http.StatusNoContent)

	if sessions := app.countUnrevokedSessions(bruno); sessions != 0 {
		t.Errorf("unrevoked sessions = %d, want 0", sessions)
	}
	expectError(t, app.do(request{method: "GET", path: "/api/core/v1/me", accessToken: brunoSignIn.AccessToken}), http.StatusUnauthorized, "invalid_token")
	for _, member := range app.listMembers(admin.AccessToken) {
		if member.Token == bruno.Token {
			t.Errorf("removed member still listed: %+v", member)
		}
	}
	expectError(t, app.do(request{method: "GET", path: "/api/core/v1/users/" + bruno.Token, accessToken: admin.AccessToken}), http.StatusNotFound, "user_not_found")
	resp, _ := app.invite(admin.AccessToken, "bruno@example.com", "member")
	expectStatus(t, resp, http.StatusCreated)
}

func TestMemberGuards(t *testing.T) {
	app := newTestApp(t)
	ana := app.createUser("ana@example.com", "Ana", "admin")
	admin := app.loginNative("ana@example.com")
	app.createRole("member-manager", "core.user.read", "core.member.read", "core.member.manage")
	app.createUser("manager@example.com", "Manager", "member-manager")
	manager := app.loginNative("manager@example.com")
	bruno := app.createUser("bruno@example.com", "Bruno", "admin")
	carla := app.createUser("carla@example.com", "Carla", "member")
	carlaSignIn := app.loginNative("carla@example.com")
	gone := app.createUser("gone@example.com", "Gone", "member")
	app.removeDirectly(gone)

	deactivate := map[string]any{"status": "deactivated"}
	cases := []struct {
		name        string
		accessToken string
		memberToken string
		body        map[string]any
		status      int
		code        string
	}{
		{"patch self", admin.AccessToken, ana.Token, deactivate, http.StatusForbidden, "cannot_manage_self"},
		{"remove self", admin.AccessToken, ana.Token, nil, http.StatusForbidden, "cannot_manage_self"},
		{"unknown member", admin.AccessToken, "unknown-token", deactivate, http.StatusNotFound, "member_not_found"},
		{"removed member", admin.AccessToken, gone.Token, nil, http.StatusNotFound, "member_not_found"},
		{"more powerful target", manager.AccessToken, bruno.Token, deactivate, http.StatusForbidden, "member_not_manageable"},
		{"remove more powerful target", manager.AccessToken, bruno.Token, nil, http.StatusForbidden, "member_not_manageable"},
		{"more powerful role", manager.AccessToken, carla.Token, map[string]any{"role": app.roleToken("admin"), "status": "deactivated"}, http.StatusForbidden, "role_not_assignable"},
		{"without permission", carlaSignIn.AccessToken, bruno.Token, deactivate, http.StatusForbidden, "forbidden"},
		{"remove without permission", carlaSignIn.AccessToken, bruno.Token, nil, http.StatusForbidden, "forbidden"},
	}
	for _, testCase := range cases {
		var resp response
		if testCase.body == nil {
			resp = app.removeMember(testCase.accessToken, testCase.memberToken)
		} else {
			resp = app.patchMember(testCase.accessToken, testCase.memberToken, testCase.body)
		}
		if resp.status != testCase.status || resp.errorCode(t) != testCase.code {
			t.Errorf("%s: %d %s, want %d %s", testCase.name, resp.status, resp.body, testCase.status, testCase.code)
		}
	}
	for _, member := range app.listMembers(admin.AccessToken) {
		if member.Status != "active" {
			t.Errorf("member changed by a refused request: %+v", member)
		}
	}
	if member := app.findMember(admin.AccessToken, carla.Token); member.Role.Name != "member" {
		t.Errorf("role changed by a refused request: %+v", member)
	}
}

func TestListMembersRequiresPermission(t *testing.T) {
	app := newTestApp(t)
	app.createUser("bruno@example.com", "Bruno", "member")

	resp := app.do(request{method: "GET", path: "/api/core/v1/members", accessToken: app.loginNative("bruno@example.com").AccessToken})

	expectError(t, resp, http.StatusForbidden, "forbidden")
}
