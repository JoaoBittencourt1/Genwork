package access_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/core/internal/access"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform/dbtest"
	"github.com/JoaoBittencourt1/Genwork/core/internal/users/userstest"
)

func TestFindUserPermissionsDropsRestrictedModulePermissions(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	registerModule(t, db, "alpha", "alpha.board.read")
	registerModule(t, db, "beta", "beta.report.read")
	user := userstest.CreateUser(t, db, "ana@example.com", "member")
	service := access.NewService(db)

	unrestricted, err := service.FindUserPermissions(ctx, user.ID)
	if err != nil {
		t.Fatalf("find unrestricted permissions: %v", err)
	}
	if !slices.Equal(unrestricted, []string{"alpha.board.read", "beta.report.read", "core.user.read"}) {
		t.Errorf("unrestricted permissions = %v", unrestricted)
	}

	if err := (access.ModuleRestrictionRepository{}).Replace(ctx, db, user.ID, []string{"alpha"}, restrictionTime); err != nil {
		t.Fatalf("restrict: %v", err)
	}
	restricted, err := service.FindUserPermissions(ctx, user.ID)
	if err != nil {
		t.Fatalf("find restricted permissions: %v", err)
	}
	if !slices.Equal(restricted, []string{"beta.report.read", "core.user.read"}) {
		t.Errorf("restricted permissions = %v", restricted)
	}
}

func TestFindUserPermissionsIgnoresRetiredPermissions(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	registerModule(t, db, "alpha", "alpha.board.read")
	user := userstest.CreateUser(t, db, "ana@example.com", "member")
	if _, err := db.Exec(ctx, `UPDATE permissions SET retired_at = now() WHERE name = 'alpha.board.read'`); err != nil {
		t.Fatalf("retire permission: %v", err)
	}

	permissions, err := access.NewService(db).FindUserPermissions(ctx, user.ID)

	if err != nil {
		t.Fatalf("find permissions: %v", err)
	}
	if !slices.Equal(permissions, []string{"core.user.read"}) {
		t.Errorf("permissions = %v", permissions)
	}
}

func TestFindUserPermissionsIsEmptyForRoleWithoutGrants(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	if _, err := db.Exec(ctx, `INSERT INTO roles (name) VALUES ('guest')`); err != nil {
		t.Fatalf("create role: %v", err)
	}
	user := userstest.CreateUser(t, db, "guest@example.com", "guest")

	permissions, err := access.NewService(db).FindUserPermissions(ctx, user.ID)

	if err != nil || permissions == nil || len(permissions) != 0 {
		t.Errorf("permissions = %#v, err = %v; want an empty, non-nil list", permissions, err)
	}
}

func TestFindRestrictedModuleNames(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	registerModule(t, db, "alpha")
	user := userstest.CreateUser(t, db, "ana@example.com", "member")
	if err := (access.ModuleRestrictionRepository{}).Replace(ctx, db, user.ID, []string{"alpha"}, restrictionTime); err != nil {
		t.Fatalf("restrict: %v", err)
	}

	names, err := access.NewService(db).FindRestrictedModuleNames(ctx, user.ID)

	if err != nil || !slices.Equal(names, []string{"alpha"}) {
		t.Errorf("restricted module names = %v, err = %v", names, err)
	}
}

func TestRequireAnyPermissionAllowsAnyListedPermission(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	member := userstest.CreateUser(t, db, "ana@example.com", "member")
	service := access.NewService(db)
	reached := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	request := httptest.NewRequest("GET", "/", nil)
	request = request.WithContext(platform.WithPrincipal(request.Context(), platform.Principal{UserID: member.ID, RoleID: member.RoleID}))

	cases := map[string]struct {
		permissions []string
		status      int
	}{
		"second permission granted": {permissions: []string{access.PermissionInviteCreate, access.PermissionUserRead}, status: http.StatusNoContent},
		"none granted":              {permissions: []string{access.PermissionInviteCreate, access.PermissionMemberManage}, status: http.StatusForbidden},
	}
	for name, testCase := range cases {
		recorder := httptest.NewRecorder()
		service.RequireAnyPermission(testCase.permissions, reached).ServeHTTP(recorder, request)
		if recorder.Code != testCase.status {
			t.Errorf("%s: status = %d, want %d", name, recorder.Code, testCase.status)
		}
	}
}
