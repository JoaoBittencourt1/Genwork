package app_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

const systemCleanupPath = "/api/core/v1/system/cleanup"

type cleanupBody struct {
	Images     []map[string]any `json:"images"`
	TotalBytes int64            `json:"totalBytes"`
	Removed    []map[string]any `json:"removed"`
	Skipped    []map[string]any `json:"skipped"`
	FreedBytes int64            `json:"freedBytes"`
}

func decodeCleanup(t *testing.T, resp response) cleanupBody {
	t.Helper()
	var body cleanupBody
	if err := json.Unmarshal(resp.body, &body); err != nil {
		t.Fatalf("decode %s: %v", resp.body, err)
	}
	return body
}

func TestAdminSeesTheOutdatedImagesOnTheServer(t *testing.T) {
	updater := newFakeUpdater(t)
	updater.answer(t, "GET /v1/cleanup", http.StatusOK, "cleanup_preview.json")
	app := newTestApp(t, withUpdater(updater))
	admin := app.signedInAdmin()

	resp := app.do(request{method: "GET", path: systemCleanupPath, accessToken: admin.AccessToken})

	expectStatus(t, resp, http.StatusOK)
	body := decodeCleanup(t, resp)
	if len(body.Images) != 2 || body.TotalBytes != 125829120 || body.Images[0]["reference"] != "ghcr.io/acme/genwork-core:0.4.0" {
		t.Errorf("body = %+v", body)
	}
	if calls := updater.recordedCalls(); len(calls) != 1 || calls[0].authorization != "Bearer "+testUpdaterSecret {
		t.Errorf("calls = %+v", calls)
	}
}

func TestAdminRemovesTheOutdatedImages(t *testing.T) {
	updater := newFakeUpdater(t)
	updater.answer(t, "POST /v1/cleanup", http.StatusOK, "cleanup_report.json")
	app := newTestApp(t, withUpdater(updater))
	admin := app.signedInAdmin()

	resp := app.do(request{method: "POST", path: systemCleanupPath, accessToken: admin.AccessToken})

	expectStatus(t, resp, http.StatusOK)
	if body := decodeCleanup(t, resp); len(body.Removed) != 1 || len(body.Skipped) != 1 || body.FreedBytes != 73400320 {
		t.Errorf("body = %+v", body)
	}
}

func TestCleanupWaitsForAnUpdateAndSaysWhenTheUpdaterIsDown(t *testing.T) {
	updater := newFakeUpdater(t)
	updater.refuse("POST /v1/cleanup", http.StatusConflict, "update_in_progress")
	app := newTestApp(t, withUpdater(updater))
	admin := app.signedInAdmin()

	expectError(t, app.do(request{method: "POST", path: systemCleanupPath, accessToken: admin.AccessToken}), http.StatusConflict, "update_in_progress")
	expectError(t, app.do(request{method: "GET", path: systemCleanupPath, accessToken: admin.AccessToken}), http.StatusServiceUnavailable, "updater_unavailable")
}

func TestCleanupNeedsTheSystemUpdatePermission(t *testing.T) {
	updater := newFakeUpdater(t)
	app := newTestApp(t, withUpdater(updater))
	app.createUser("bruno@example.com", "Bruno", "member")
	member := app.loginNative("bruno@example.com")

	for _, method := range []string{"GET", "POST"} {
		expectError(t, app.do(request{method: method, path: systemCleanupPath, accessToken: member.AccessToken}), http.StatusForbidden, "forbidden")
	}
	if calls := updater.recordedCalls(); len(calls) != 0 {
		t.Errorf("the updater must receive nothing: %+v", calls)
	}
}
