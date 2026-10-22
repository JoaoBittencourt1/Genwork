package updates

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/cli/internal/releases/releasestest"
)

const updaterSecret = "updater-secret-0123456789abcdef"

type updaterFixture struct {
	*runnerFixture
	server   *httptest.Server
	replaced atomic.Int32
}

func newUpdaterFixture(t *testing.T) *updaterFixture {
	t.Helper()
	fixture := &updaterFixture{runnerFixture: newRunnerFixture(t)}
	service := NewUpdaterService(fixture.runner, func(context.Context, UpdateStatus) { fixture.replaced.Add(1) })
	fixture.server = httptest.NewServer(NewUpdaterHandler(service, updaterSecret))
	t.Cleanup(fixture.server.Close)
	return fixture
}

func (fixture *updaterFixture) call(t *testing.T, method, path, secret, body string) (int, map[string]any) {
	t.Helper()
	request, err := http.NewRequest(method, fixture.server.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if secret != "" {
		request.Header.Set("Authorization", "Bearer "+secret)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var decoded map[string]any
	_ = json.NewDecoder(response.Body).Decode(&decoded)
	return response.StatusCode, decoded
}

func errorCodeOf(body map[string]any) string {
	payload, _ := body["error"].(map[string]any)
	code, _ := payload["code"].(string)
	return code
}

func TestUpdaterRefusesRequestsWithoutTheSecret(t *testing.T) {
	fixture := newUpdaterFixture(t)

	for _, secret := range []string{"", "wrong-secret-0123456789abcdef", updaterSecret + "x"} {
		for _, route := range [][2]string{{"GET", "/v1/status"}, {"POST", "/v1/check"}, {"POST", "/v1/updates"}} {
			status, body := fixture.call(t, route[0], route[1], secret, `{"version":"0.5.2"}`)
			if status != http.StatusUnauthorized || errorCodeOf(body) != "invalid_updater_secret" {
				t.Errorf("%s %s with %q = %d %v", route[0], route[1], secret, status, body)
			}
		}
	}
	if fixture.github.LatestRequests() != 0 || len(fixture.installation.recordedCalls()) != 0 {
		t.Error("a refused request must do nothing")
	}
}

func TestUpdaterServesTheStatusAndChecks(t *testing.T) {
	fixture := newUpdaterFixture(t)

	status, body := fixture.call(t, "GET", "/v1/status", updaterSecret, "")
	if status != http.StatusOK || body["installedVersion"] != installedVersion || body["state"] != "idle" || body["latest"] != nil || body["updatable"] != true {
		t.Fatalf("status = %d %v", status, body)
	}
	status, body = fixture.call(t, "POST", "/v1/check", updaterSecret, "")
	latest, _ := body["latest"].(map[string]any)
	if status != http.StatusOK || latest["version"] != releasestest.LatestVersion || latest["notesUrl"] == nil || latest["publishedAt"] == nil || body["checkedAt"] == nil {
		t.Errorf("check = %d %v", status, body)
	}
}

func TestUpdaterReportsAnUnreachableReleaseChannel(t *testing.T) {
	fixture := newUpdaterFixture(t)
	fixture.github.RateLimit()

	status, body := fixture.call(t, "POST", "/v1/check", updaterSecret, "")

	if status != http.StatusBadGateway || errorCodeOf(body) != "release_channel_rate_limited" {
		t.Errorf("check = %d %v", status, body)
	}
}

func TestUpdaterAppliesInTheBackgroundAndReplacesItselfLast(t *testing.T) {
	fixture := newUpdaterFixture(t)
	fixture.call(t, "POST", "/v1/check", updaterSecret, "")

	status, body := fixture.call(t, "POST", "/v1/updates", updaterSecret, `{"version":"0.5.2"}`)

	if status != http.StatusAccepted || body["state"] != "updating" || body["targetVersion"] != releasestest.LatestVersion {
		t.Fatalf("apply = %d %v", status, body)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, current := fixture.call(t, "GET", "/v1/status", updaterSecret, ""); current["state"] == "succeeded" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	_, final := fixture.call(t, "GET", "/v1/status", updaterSecret, "")
	if final["state"] != "succeeded" || final["installedVersion"] != releasestest.LatestVersion {
		t.Fatalf("final = %v", final)
	}
	deadline = time.Now().Add(time.Second)
	for fixture.replaced.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if fixture.replaced.Load() != 1 {
		t.Error("the updater must replace its own container after a successful update")
	}
}

func TestUpdaterRefusesConflictingUpdates(t *testing.T) {
	fixture := newUpdaterFixture(t)

	status, body := fixture.call(t, "POST", "/v1/updates", updaterSecret, `{"version":"0.5.2"}`)
	if status != http.StatusConflict || errorCodeOf(body) != CodeVersionMismatch {
		t.Errorf("before a check = %d %v", status, body)
	}
	fixture.call(t, "POST", "/v1/check", updaterSecret, "")
	if status, body = fixture.call(t, "POST", "/v1/updates", updaterSecret, `{"version":"0.5.1"}`); status != http.StatusConflict || errorCodeOf(body) != CodeVersionMismatch {
		t.Errorf("stale version = %d %v", status, body)
	}
	held, err := fixture.newRunner().Begin(releasestest.LatestVersion)
	if err != nil {
		t.Fatal(err)
	}
	if status, body = fixture.call(t, "POST", "/v1/updates", updaterSecret, `{"version":"0.5.2"}`); status != http.StatusConflict || errorCodeOf(body) != CodeUpdateInProgress {
		t.Errorf("while updating = %d %v", status, body)
	}
	if status, body = fixture.call(t, "POST", "/v1/check", updaterSecret, ""); status != http.StatusConflict || errorCodeOf(body) != CodeUpdateInProgress {
		t.Errorf("check while updating = %d %v", status, body)
	}
	held.Run(context.Background())
	if status, body = fixture.call(t, "POST", "/v1/updates", updaterSecret, `{"version":"0.5.2"}`); status != http.StatusConflict || errorCodeOf(body) != CodeNoNewerRelease {
		t.Errorf("already updated = %d %v", status, body)
	}
}

func TestUpdaterValidatesTheRequestedVersion(t *testing.T) {
	fixture := newUpdaterFixture(t)

	for _, body := range []string{`{"version":"latest"}`, `{"version":""}`, `{}`, `{"version":"0.5.2","force":true}`, `not json`} {
		status, response := fixture.call(t, "POST", "/v1/updates", updaterSecret, body)
		if status != http.StatusBadRequest {
			t.Errorf("%s = %d %v", body, status, response)
		}
	}
}

func TestUpdaterReportsDevelopmentCheckoutsAsNotUpdatable(t *testing.T) {
	fixture := newUpdaterFixture(t)
	fixture.installation.repository = true
	fixture.call(t, "POST", "/v1/check", updaterSecret, "")

	_, status := fixture.call(t, "GET", "/v1/status", updaterSecret, "")
	code, body := fixture.call(t, "POST", "/v1/updates", updaterSecret, `{"version":"0.5.2"}`)

	if status["updatable"] != false || status["reason"] != ReasonDevelopmentInstallation {
		t.Errorf("status = %v", status)
	}
	if code != http.StatusConflict || errorCodeOf(body) != ReasonDevelopmentInstallation {
		t.Errorf("apply = %d %v", code, body)
	}
}
