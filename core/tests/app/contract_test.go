package app_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// The SDK stubs core from these recorded responses (rule 62); this test fails when core's real responses drift from them.
const sdkFixturesDir = "../../../sdk/go/coretest/fixtures"

func readSDKFixture(t *testing.T, name string) any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(sdkFixturesDir, name))
	if err != nil {
		t.Fatalf("read SDK fixture %s: %v", name, err)
	}
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode SDK fixture %s: %v", name, err)
	}
	return decoded
}

func expectSameShape(t *testing.T, path string, live, recorded any) {
	t.Helper()
	switch recordedValue := recorded.(type) {
	case map[string]any:
		liveValue, ok := live.(map[string]any)
		if !ok {
			t.Errorf("%s: live value %v is not an object", path, live)
			return
		}
		if liveKeys, recordedKeys := sortedKeys(liveValue), sortedKeys(recordedValue); !equalStrings(liveKeys, recordedKeys) {
			t.Errorf("%s: live keys %v, recorded keys %v", path, liveKeys, recordedKeys)
			return
		}
		for key := range recordedValue {
			expectSameShape(t, path+"."+key, liveValue[key], recordedValue[key])
		}
	case []any:
		liveValue, ok := live.([]any)
		if !ok {
			t.Errorf("%s: live value %v is not an array", path, live)
			return
		}
		if len(liveValue) > 0 && len(recordedValue) > 0 {
			expectSameShape(t, path+"[0]", liveValue[0], recordedValue[0])
		}
	default:
		if jsonKind(live) != jsonKind(recorded) {
			t.Errorf("%s: live %T, recorded %T", path, live, recorded)
		}
	}
}

func sortedKeys(object map[string]any) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func jsonKind(value any) string {
	switch value.(type) {
	case string:
		return "string"
	case bool:
		return "bool"
	case float64:
		return "number"
	case nil:
		return "null"
	default:
		return "other"
	}
}

func liveBody(t *testing.T, resp response) any {
	t.Helper()
	var decoded any
	resp.decode(t, &decoded)
	return decoded
}

func TestCoreResponsesMatchSDKContractFixtures(t *testing.T) {
	app := newTestApp(t)
	ana := app.createUser("ana@example.com", "Ana", "member")
	signIn := app.loginNative("ana@example.com")
	incompatible := gentasksManifest()
	incompatible.Core = ">=2.0.0"

	live := map[string]response{
		"registered_module.json":         app.registerModule(gentasksManifest()),
		"incompatible_core_version.json": app.registerModule(incompatible),
		"jwks.json":                      app.do(request{method: "GET", path: "/api/core/v1/auth/jwks"}),
		"directory_user.json":            app.do(request{method: "GET", path: "/api/core/v1/users/" + ana.Token, accessToken: signIn.AccessToken}),
		"user_not_found.json":            app.do(request{method: "GET", path: "/api/core/v1/users/unknown", accessToken: signIn.AccessToken}),
		"capabilities.json":              app.do(request{method: "GET", path: "/api/core/v1/capabilities", accessToken: signIn.AccessToken}),
	}
	expectStatus(t, live["capabilities.json"], http.StatusOK)

	fixtures, err := os.ReadDir(sdkFixturesDir)
	if err != nil {
		t.Fatalf("list SDK fixtures: %v", err)
	}
	for _, fixture := range fixtures {
		name := fixture.Name()
		t.Run(name, func(t *testing.T) {
			resp, found := live[name]
			if !found {
				t.Fatalf("no core response is compared with fixture %s", name)
			}
			expectSameShape(t, name, liveBody(t, resp), readSDKFixture(t, name))
		})
	}
}
