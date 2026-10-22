package app_test

import (
	"net/http"
	"strings"
	"testing"
)

const (
	updateStatusPath = "/api/core/v1/system/update"
	updateCheckPath  = "/api/core/v1/system/update/check"
)

func TestSystemUpdateNeedsThePermissionAndNeverReachesTheUpdaterWithoutIt(t *testing.T) {
	updater := newFakeUpdater(t)
	updater.answer(t, "GET /v1/status", http.StatusOK, "status_idle.json")
	app := newTestApp(t, withUpdater(updater))
	app.createUser("bruno@example.com", "Bruno", "member")
	member := app.loginNative("bruno@example.com")

	for _, route := range [][2]string{{"GET", updateStatusPath}, {"POST", updateCheckPath}, {"POST", updateStatusPath}} {
		resp := app.do(request{method: route[0], path: route[1], accessToken: member.AccessToken, body: map[string]any{"version": "0.5.2"}})
		expectError(t, resp, http.StatusForbidden, "forbidden")
		expectError(t, app.do(request{method: route[0], path: route[1]}), http.StatusUnauthorized, "invalid_token")
	}
	if calls := updater.recordedCalls(); len(calls) != 0 {
		t.Errorf("the updater must receive nothing: %+v", calls)
	}
}

func TestAdminReadsTheUpdateStatusThroughCore(t *testing.T) {
	updater := newFakeUpdater(t)
	updater.answer(t, "GET /v1/status", http.StatusOK, "status_idle.json")
	app := newTestApp(t, withUpdater(updater))
	admin := app.signedInAdmin()

	resp, status := app.updateStatus("GET", updateStatusPath, admin.AccessToken, nil)

	expectStatus(t, resp, http.StatusOK)
	if status["installedVersion"] != "0.5.0" || status["updatable"] != true || status["state"] != "idle" || status["latest"] != nil {
		t.Errorf("status = %v", status)
	}
	if _, present := status["latest"]; !present {
		t.Error("latest must be present, as null, when no release is known")
	}
	calls := updater.recordedCalls()
	if len(calls) != 1 || calls[0].authorization != "Bearer "+testUpdaterSecret {
		t.Errorf("calls = %+v", calls)
	}
}

func TestAdminChecksForUpdates(t *testing.T) {
	updater := newFakeUpdater(t)
	updater.answer(t, "POST /v1/check", http.StatusOK, "status_available.json")
	app := newTestApp(t, withUpdater(updater))
	admin := app.signedInAdmin()

	resp, status := app.updateStatus("POST", updateCheckPath, admin.AccessToken, nil)

	expectStatus(t, resp, http.StatusOK)
	latest, _ := status["latest"].(map[string]any)
	if latest["version"] != "0.5.2" || latest["notesUrl"] != "https://github.com/acme/genwork/releases/tag/v0.5.2" ||
		latest["publishedAt"] != "2026-10-20T12:00:00Z" || status["checkedAt"] != "2026-10-22T13:55:00Z" {
		t.Errorf("status = %v", status)
	}
}

func TestAdminStartsAnUpdate(t *testing.T) {
	updater := newFakeUpdater(t)
	updater.answer(t, "POST /v1/updates", http.StatusAccepted, "status_updating.json")
	app := newTestApp(t, withUpdater(updater))
	admin := app.signedInAdmin()

	resp, status := app.updateStatus("POST", updateStatusPath, admin.AccessToken, map[string]any{"version": "0.5.2"})

	expectStatus(t, resp, http.StatusAccepted)
	if status["state"] != "updating" || status["targetVersion"] != "0.5.2" || status["step"] != "download" || status["startedAt"] == nil {
		t.Errorf("status = %v", status)
	}
	calls := updater.recordedCalls()
	if len(calls) != 1 || calls[0].path != "/v1/updates" || strings.TrimSpace(calls[0].body) != `{"version":"0.5.2"}` {
		t.Errorf("calls = %+v", calls)
	}
}

func TestUpdateStatusShowsAFailureWithoutServerPaths(t *testing.T) {
	updater := newFakeUpdater(t)
	updater.answer(t, "GET /v1/status", http.StatusOK, "status_rollback_failed.json")
	app := newTestApp(t, withUpdater(updater))
	admin := app.signedInAdmin()

	resp, status := app.updateStatus("GET", updateStatusPath, admin.AccessToken, nil)

	expectStatus(t, resp, http.StatusOK)
	failure, _ := status["failure"].(map[string]any)
	if failure["code"] != "migration_failed" || failure["step"] != "migrate" || failure["rolledBack"] != false || status["finishedAt"] == nil {
		t.Errorf("status = %v", status)
	}
	if strings.Contains(string(resp.body), "backups") || strings.Contains(string(resp.body), "previousVersion") {
		t.Errorf("the response must carry only the contract's fields: %s", resp.body)
	}
}

func TestUpdateStatusOfADevelopmentCheckout(t *testing.T) {
	updater := newFakeUpdater(t)
	updater.answer(t, "GET /v1/status", http.StatusOK, "status_development.json")
	app := newTestApp(t, withUpdater(updater))
	admin := app.signedInAdmin()

	_, status := app.updateStatus("GET", updateStatusPath, admin.AccessToken, nil)

	if status["updatable"] != false || status["reason"] != "development_installation" {
		t.Errorf("status = %v", status)
	}
}

func TestUpdaterRefusalsMapToAPIErrors(t *testing.T) {
	cases := []struct {
		updaterStatus int
		updaterCode   string
		status        int
		code          string
	}{
		{http.StatusConflict, "version_mismatch", http.StatusConflict, "version_mismatch"},
		{http.StatusConflict, "update_in_progress", http.StatusConflict, "update_in_progress"},
		{http.StatusConflict, "no_newer_release", http.StatusConflict, "no_newer_release"},
		{http.StatusConflict, "development_installation", http.StatusConflict, "development_installation"},
		{http.StatusBadGateway, "release_channel_unreachable", http.StatusBadGateway, "release_channel_unreachable"},
		{http.StatusUnauthorized, "invalid_updater_secret", http.StatusServiceUnavailable, "updater_unavailable"},
		{http.StatusInternalServerError, "internal_error", http.StatusServiceUnavailable, "updater_unavailable"},
	}
	for _, testCase := range cases {
		t.Run(testCase.updaterCode, func(t *testing.T) {
			updater := newFakeUpdater(t)
			updater.refuse("POST /v1/updates", testCase.updaterStatus, testCase.updaterCode)
			app := newTestApp(t, withUpdater(updater))
			admin := app.signedInAdmin()

			resp, _ := app.updateStatus("POST", updateStatusPath, admin.AccessToken, map[string]any{"version": "0.5.2"})

			expectError(t, resp, testCase.status, testCase.code)
		})
	}
}

func TestAnUnreachableUpdaterIsUnavailable(t *testing.T) {
	app := newTestApp(t)
	admin := app.signedInAdmin()

	for _, route := range [][2]string{{"GET", updateStatusPath}, {"POST", updateCheckPath}} {
		resp, _ := app.updateStatus(route[0], route[1], admin.AccessToken, nil)
		expectError(t, resp, http.StatusServiceUnavailable, "updater_unavailable")
	}
}

func TestStartUpdateValidatesTheVersionBeforeCallingTheUpdater(t *testing.T) {
	updater := newFakeUpdater(t)
	app := newTestApp(t, withUpdater(updater))
	admin := app.signedInAdmin()

	for _, body := range []map[string]any{{"version": "latest"}, {"version": "v0.5.2"}, {"version": ""}, {}} {
		resp, _ := app.updateStatus("POST", updateStatusPath, admin.AccessToken, body)
		expectError(t, resp, http.StatusBadRequest, "validation_failed")
	}
	if calls := updater.recordedCalls(); len(calls) != 0 {
		t.Errorf("calls = %+v", calls)
	}
}
