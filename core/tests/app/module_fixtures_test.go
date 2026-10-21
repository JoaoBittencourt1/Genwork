package app_test

const testModuleSecret = "test-module-secret-0123456789abcdef"

type permissionFixture struct {
	Name         string   `json:"name"`
	DefaultRoles []string `json:"defaultRoles"`
}

type requirementFixture struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

type manifestFixture struct {
	ID          string               `json:"id"`
	Version     string               `json:"version"`
	Core        string               `json:"core"`
	APIPrefix   string               `json:"apiPrefix"`
	Requires    []requirementFixture `json:"requires"`
	Permissions []permissionFixture  `json:"permissions"`
}

func gentasksManifest() manifestFixture {
	return manifestFixture{
		ID:        "gentasks",
		Version:   "0.1.0",
		Core:      ">=1.0.0 <2.0.0",
		APIPrefix: "/api/gentasks/v1",
		Requires:  []requirementFixture{},
		Permissions: []permissionFixture{
			{Name: "gentasks.board.read", DefaultRoles: []string{"admin", "member"}},
			{Name: "gentasks.task.delete", DefaultRoles: []string{"admin"}},
		},
	}
}

func (app *testApp) registerModule(manifest manifestFixture) response {
	app.t.Helper()
	return app.do(request{
		method: "PUT", path: "/internal/v1/modules/" + manifest.ID, body: manifest, moduleSecret: testModuleSecret,
	})
}
