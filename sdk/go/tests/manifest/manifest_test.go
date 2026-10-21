package manifest_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/manifest"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

func validManifest(t *testing.T) manifest.ModuleManifest {
	t.Helper()
	parsed, err := manifest.ParseManifest(readFixture(t, "valid.yaml"))
	if err != nil {
		t.Fatalf("parse valid manifest: %v", err)
	}
	return parsed
}

func TestParseManifestReadsEveryFact(t *testing.T) {
	parsed := validManifest(t)

	if parsed.ID != "gentasks" || parsed.Version != "0.1.0" || parsed.Core != ">=1.0.0 <2.0.0" || parsed.APIPrefix != "/api/gentasks/v1" {
		t.Errorf("manifest = %+v", parsed)
	}
	if len(parsed.Requires) != 1 || parsed.Requires[0].ID != "identity" || parsed.Requires[0].Version != ">=1.2.0" {
		t.Errorf("requires = %+v", parsed.Requires)
	}
	if len(parsed.Permissions) != 2 || parsed.Permissions[1].DefaultRoles[0] != "admin" {
		t.Errorf("permissions = %+v", parsed.Permissions)
	}
	if !parsed.DeclaresPermission("gentasks.board.read") || parsed.DeclaresPermission("gentasks.board.manage") {
		t.Error("DeclaresPermission mismatch")
	}
}

func TestParseManifestRejectsMalformedFiles(t *testing.T) {
	for _, name := range []string{"unknown_field.yaml", "not_yaml.yaml"} {
		if _, err := manifest.ParseManifest(readFixture(t, name)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestValidateRejectsInvalidFacts(t *testing.T) {
	cases := map[string]struct {
		mutate func(*manifest.ModuleManifest)
		field  string
	}{
		"bad id":                   {func(m *manifest.ModuleManifest) { m.ID = "Gen Tasks" }, "id"},
		"bad version":              {func(m *manifest.ModuleManifest) { m.Version = "1.0" }, "version"},
		"bad core range":           {func(m *manifest.ModuleManifest) { m.Core = "soon" }, "core"},
		"prefix of another module": {func(m *manifest.ModuleManifest) { m.APIPrefix = "/api/kanban/v1" }, "apiPrefix"},
		"prefix without version":   {func(m *manifest.ModuleManifest) { m.APIPrefix = "/api/gentasks" }, "apiPrefix"},
		"permission outside namespace": {
			func(m *manifest.ModuleManifest) { m.Permissions[0].Name = "core.user.read" }, "permissions",
		},
		"permission without action": {
			func(m *manifest.ModuleManifest) { m.Permissions[0].Name = "gentasks.board" }, "permissions",
		},
		"duplicated permission": {
			func(m *manifest.ModuleManifest) { m.Permissions[1].Name = m.Permissions[0].Name }, "permissions",
		},
		"requirement range": {
			func(m *manifest.ModuleManifest) { m.Requires[0].Version = "latest" }, "requires",
		},
		"self requirement": {
			func(m *manifest.ModuleManifest) { m.Requires[0].ID = "gentasks" }, "requires",
		},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			parsed := validManifest(t)
			testCase.mutate(&parsed)

			err := parsed.Validate()

			if err == nil || !strings.Contains(err.Error(), testCase.field) {
				t.Errorf("error = %v, want mention of %s", err, testCase.field)
			}
		})
	}
}

func TestRegistrationRequestCarriesOnlyRegistrationFacts(t *testing.T) {
	encoded, err := json.Marshal(validManifest(t).RegistrationRequest())
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(encoded, &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, key := range []string{"id", "version", "core", "apiPrefix", "requires", "permissions"} {
		if _, found := body[key]; !found {
			t.Errorf("missing %s in %s", key, encoded)
		}
	}
	if _, found := body["events"]; found || len(body) != 6 {
		t.Errorf("unexpected keys in %s", encoded)
	}
}

func TestRegistrationRequestSendsEmptyListsInsteadOfNull(t *testing.T) {
	bare := manifest.ModuleManifest{ID: "bare", Version: "1.0.0", Core: ">=1.0.0", APIPrefix: "/api/bare/v1"}

	encoded, _ := json.Marshal(bare.RegistrationRequest())

	if !strings.Contains(string(encoded), `"requires":[]`) || !strings.Contains(string(encoded), `"permissions":[]`) {
		t.Errorf("encoded = %s", encoded)
	}
}
