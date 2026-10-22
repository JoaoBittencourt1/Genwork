package app_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/core/internal/access"
)

type registeredModuleBody struct {
	Token   string `json:"token"`
	ID      string `json:"id"`
	Version string `json:"version"`
	Enabled bool   `json:"enabled"`
}

type capabilitiesBody struct {
	Core struct {
		Version string `json:"version"`
		API     string `json:"api"`
	} `json:"core"`
	Modules []struct {
		ID        string `json:"id"`
		Version   string `json:"version"`
		APIPrefix string `json:"apiPrefix"`
	} `json:"modules"`
}

func (app *testApp) roleGrants(roleName, permission string) bool {
	app.t.Helper()
	role, err := access.RoleRepository{}.FindByName(context.Background(), app.db, roleName)
	if err != nil {
		app.t.Fatalf("find role %s: %v", roleName, err)
	}
	return role.Grants(permission)
}

func (app *testApp) capabilities() capabilitiesBody {
	app.t.Helper()
	app.createUser("caps@example.com", "Caps", "member")
	signIn := app.loginNative("caps@example.com")
	resp := app.do(request{method: "GET", path: "/api/core/v1/capabilities", accessToken: signIn.AccessToken})
	expectStatus(app.t, resp, http.StatusOK)
	var body capabilitiesBody
	resp.decode(app.t, &body)
	return body
}

func TestModuleRegistrationStoresModuleAndGrantsDefaultRoles(t *testing.T) {
	app := newTestApp(t)

	resp := app.registerModule(gentasksManifest())

	expectStatus(t, resp, http.StatusOK)
	var body registeredModuleBody
	resp.decode(t, &body)
	if len(body.Token) != 22 || body.ID != "gentasks" || body.Version != "0.1.0" || !body.Enabled {
		t.Errorf("registered module = %+v", body)
	}
	if !app.roleGrants("admin", "gentasks.board.read") || !app.roleGrants("member", "gentasks.board.read") {
		t.Error("gentasks.board.read should be granted to admin and member")
	}
	if !app.roleGrants("admin", "gentasks.task.delete") || app.roleGrants("member", "gentasks.task.delete") {
		t.Error("gentasks.task.delete should be granted to admin only")
	}
}

func TestModuleRegistrationAcceptsTheDevelopmentBuildVersion(t *testing.T) {
	app := newTestApp(t)
	manifest := gentasksManifest()
	manifest.Version = "0.0.0-dev"

	resp := app.registerModule(manifest)

	expectStatus(t, resp, http.StatusOK)
	var body registeredModuleBody
	resp.decode(t, &body)
	if body.Version != "0.0.0-dev" {
		t.Errorf("registered version = %q", body.Version)
	}
}

func TestModuleRegistrationRequiresModuleSecret(t *testing.T) {
	app := newTestApp(t)
	manifest := gentasksManifest()

	missing := app.do(request{method: "PUT", path: "/internal/v1/modules/gentasks", body: manifest})
	wrong := app.do(request{method: "PUT", path: "/internal/v1/modules/gentasks", body: manifest, moduleSecret: "wrong-secret"})

	expectError(t, missing, http.StatusUnauthorized, "invalid_module_secret")
	expectError(t, wrong, http.StatusUnauthorized, "invalid_module_secret")
	if app.roleGrants("admin", "gentasks.board.read") {
		t.Error("a rejected registration must not store permissions")
	}
}

func TestModuleRegistrationValidatesManifest(t *testing.T) {
	cases := map[string]func(*manifestFixture){
		"path and manifest id differ":  func(m *manifestFixture) { m.ID = "other" },
		"permission of another module": func(m *manifestFixture) { m.Permissions[0].Name = "kanban.issue.create" },
		"core permission":              func(m *manifestFixture) { m.Permissions[0].Name = "core.user.read" },
		"unknown default role":         func(m *manifestFixture) { m.Permissions[0].DefaultRoles = []string{"owner"} },
		"invalid version":              func(m *manifestFixture) { m.Version = "one" },
		"invalid core range":           func(m *manifestFixture) { m.Core = "not a range" },
		"api prefix of another module": func(m *manifestFixture) { m.APIPrefix = "/api/kanban/v1" },
		"api prefix without version":   func(m *manifestFixture) { m.APIPrefix = "/api/gentasks" },
		"duplicated permission":        func(m *manifestFixture) { m.Permissions[1].Name = m.Permissions[0].Name },
		"requirement with invalid range": func(m *manifestFixture) {
			m.Requires = []requirementFixture{{ID: "identity", Version: "latest"}}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			app := newTestApp(t)
			manifest := gentasksManifest()
			mutate(&manifest)

			resp := app.do(request{method: "PUT", path: "/internal/v1/modules/gentasks", body: manifest, moduleSecret: testModuleSecret})

			expectError(t, resp, http.StatusBadRequest, "validation_failed")
			if app.roleGrants("admin", "gentasks.board.read") {
				t.Error("an invalid registration must not store permissions")
			}
		})
	}
}

func TestModuleRegistrationRejectsIncompatibleCore(t *testing.T) {
	app := newTestApp(t)
	manifest := gentasksManifest()
	manifest.Core = ">=2.0.0"

	expectError(t, app.registerModule(manifest), http.StatusConflict, "incompatible_core_version")
	if len(app.capabilities().Modules) != 0 {
		t.Error("an incompatible module must not be enabled")
	}
}

func TestModuleRegistrationChecksRequirements(t *testing.T) {
	app := newTestApp(t)
	dependent := gentasksManifest()
	dependent.Requires = []requirementFixture{{ID: "identity", Version: ">=1.2.0"}}
	dependency := manifestFixture{ID: "identity", Version: "1.0.0", Core: ">=1.0.0", APIPrefix: "/api/identity/v1"}

	expectError(t, app.registerModule(dependent), http.StatusConflict, "missing_requirement")

	expectStatus(t, app.registerModule(dependency), http.StatusOK)
	expectError(t, app.registerModule(dependent), http.StatusConflict, "missing_requirement")

	dependency.Version = "1.3.0"
	expectStatus(t, app.registerModule(dependency), http.StatusOK)
	expectStatus(t, app.registerModule(dependent), http.StatusOK)
}

func TestModuleReRegistrationUpdatesVersion(t *testing.T) {
	app := newTestApp(t)
	manifest := gentasksManifest()
	expectStatus(t, app.registerModule(manifest), http.StatusOK)

	manifest.Version = "0.2.0"
	resp := app.registerModule(manifest)

	expectStatus(t, resp, http.StatusOK)
	modules := app.capabilities().Modules
	if len(modules) != 1 || modules[0].Version != "0.2.0" {
		t.Errorf("modules = %+v", modules)
	}
}

func TestModuleReRegistrationDoesNotReapplyDefaultGrants(t *testing.T) {
	app := newTestApp(t)
	expectStatus(t, app.registerModule(gentasksManifest()), http.StatusOK)
	_, err := app.db.Exec(context.Background(), `
		DELETE FROM role_permissions
		USING roles, permissions
		WHERE role_permissions.role_id = roles.id AND role_permissions.permission_id = permissions.id
		  AND roles.name = 'member' AND permissions.name = 'gentasks.board.read'`)
	if err != nil {
		t.Fatalf("revoke grant: %v", err)
	}

	expectStatus(t, app.registerModule(gentasksManifest()), http.StatusOK)

	if app.roleGrants("member", "gentasks.board.read") {
		t.Error("default grants must only be applied when the permission is first seen")
	}
}

func TestModuleReRegistrationGrantsRolesTheManifestNewlyDeclares(t *testing.T) {
	app := newTestApp(t)
	expectStatus(t, app.registerModule(gentasksManifest()), http.StatusOK)
	if app.roleGrants("member", "gentasks.task.delete") {
		t.Fatal("the first manifest grants gentasks.task.delete to admin only")
	}

	widened := gentasksManifest()
	widened.Permissions[1].DefaultRoles = []string{"admin", "member"}
	expectStatus(t, app.registerModule(widened), http.StatusOK)

	if !app.roleGrants("member", "gentasks.task.delete") || !app.roleGrants("admin", "gentasks.task.delete") {
		t.Error("a role the manifest starts declaring must receive the permission")
	}
}

func TestModuleReRegistrationNeverRevokesARoleTheManifestDropped(t *testing.T) {
	app := newTestApp(t)
	expectStatus(t, app.registerModule(gentasksManifest()), http.StatusOK)

	narrowed := gentasksManifest()
	narrowed.Permissions[0].DefaultRoles = []string{"admin"}
	expectStatus(t, app.registerModule(narrowed), http.StatusOK)
	expectStatus(t, app.registerModule(gentasksManifest()), http.StatusOK)

	if !app.roleGrants("member", "gentasks.board.read") {
		t.Error("dropping a default role keeps the grant it gave")
	}
}

func TestModulePermissionsRemovedFromManifestAreRetiredAndRevived(t *testing.T) {
	app := newTestApp(t)
	expectStatus(t, app.registerModule(gentasksManifest()), http.StatusOK)

	withoutBoardRead := gentasksManifest()
	withoutBoardRead.Permissions = withoutBoardRead.Permissions[1:]
	expectStatus(t, app.registerModule(withoutBoardRead), http.StatusOK)

	if app.roleGrants("admin", "gentasks.board.read") || app.roleGrants("member", "gentasks.board.read") {
		t.Fatal("a retired permission must not be granted")
	}

	expectStatus(t, app.registerModule(gentasksManifest()), http.StatusOK)
	if !app.roleGrants("admin", "gentasks.board.read") || !app.roleGrants("member", "gentasks.board.read") {
		t.Error("a revived permission keeps its previous grants")
	}
}

func TestCapabilitiesWithoutModules(t *testing.T) {
	app := newTestApp(t)

	body := app.capabilities()

	if len(body.Modules) != 0 || body.Core.Version != "test" || body.Core.API != "1.0.0" {
		t.Errorf("capabilities = %+v", body)
	}
}

func TestCapabilitiesListEnabledModules(t *testing.T) {
	app := newTestApp(t)
	expectStatus(t, app.registerModule(gentasksManifest()), http.StatusOK)

	modules := app.capabilities().Modules

	if len(modules) != 1 || modules[0].ID != "gentasks" || modules[0].Version != "0.1.0" || modules[0].APIPrefix != "/api/gentasks/v1" {
		t.Errorf("modules = %+v", modules)
	}
}

func TestCapabilitiesRequireAuthentication(t *testing.T) {
	app := newTestApp(t)
	expectError(t, app.do(request{method: "GET", path: "/api/core/v1/capabilities"}), http.StatusUnauthorized, "invalid_token")
}
