package app_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/access"
)

const runnersCleanupPath = "/api/genagents/v1/runners/cleanup"

func (app *testApp) heartbeatAnswer(runner testRunner, body map[string]any) map[string]any {
	app.t.Helper()
	payload := map[string]any{"containerEngine": "docker", "sandboxImage": "genwork-sandbox:test", "tools": map[string]string{"claude": "1"}, "maxConcurrency": 2}
	for key, value := range body {
		payload[key] = value
	}
	resp := app.runnerDo(runner.Secret, "POST", "/api/genagents/v1/runner/heartbeat", payload)
	expectStatus(app.t, resp, http.StatusOK)
	var answer map[string]any
	resp.decode(app.t, &answer)
	return answer
}

func TestFreeingSpaceAsksEveryRunnerAndReportsWhatItFreed(t *testing.T) {
	app := newTestApp(t)
	runner := app.registerRunner("build-box", heartbeatOptions{tools: []string{"claude"}})
	if answer := app.heartbeatAnswer(runner, nil); answer["cleanupRequested"] != false {
		t.Fatalf("a runner was asked to clean up before anyone asked: %v", answer)
	}

	resp := app.do(request{method: "POST", path: runnersCleanupPath})

	expectStatus(t, resp, http.StatusOK)
	var result map[string]any
	resp.decode(t, &result)
	if result["runnersAsked"] != float64(1) {
		t.Errorf("result = %v", result)
	}
	if body := app.runnerByToken(runner.Token); !body.CleanupPending {
		t.Errorf("runner = %+v", body)
	}
	if answer := app.heartbeatAnswer(runner, nil); answer["cleanupRequested"] != true {
		t.Fatalf("answer = %v", answer)
	}
	app.clock.Advance(time.Minute)
	if answer := app.heartbeatAnswer(runner, map[string]any{"cleanup": map[string]any{"freedBytes": 3 << 30}}); answer["cleanupRequested"] != false {
		t.Errorf("the request outlived the runner's report: %v", answer)
	}
	body := app.runnerByToken(runner.Token)
	if body.CleanupPending || body.LastCleanupFreedBytes == nil || *body.LastCleanupFreedBytes != 3<<30 || body.LastCleanupAt == nil {
		t.Errorf("runner = %+v", body)
	}
}

func TestFreeingSpaceRemovesOldSignInsAndAuthorizations(t *testing.T) {
	app := newTestApp(t)
	runner, account, old := app.awaitingCode()
	expectNoContent(t, app.reportSignIn(runner, old.Token, map[string]any{"state": "failed", "message": "boom"}))
	app.clock.Advance(8 * 24 * time.Hour)
	recent := app.startSignIn(account.Token)
	ctx := context.Background()
	server := app.createSentry()
	_, err := app.db.Exec(ctx, `
		INSERT INTO mcp_server_oauth_authorizations (token, mcp_server_id, state_hash, code_verifier_ciphertext, server_url, token_endpoint, resource,
		       client_id, expires_at, created_at)
		SELECT 'old-authorization', id, 'hash', 'verifier', 'https://x', 'https://x/token', 'https://x', 'client', $2, $2 FROM mcp_servers WHERE token = $1`,
		server.Token, app.clock.Now().Add(-48*time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	resp := app.do(request{method: "POST", path: runnersCleanupPath})

	var result map[string]any
	resp.decode(t, &result)
	if result["removedSignIns"] != float64(1) || result["removedAuthorizations"] != float64(1) {
		t.Errorf("result = %v", result)
	}
	if left := app.countRows(`SELECT count(*) FROM account_sign_ins`); left != 1 {
		t.Errorf("sign-ins left = %d", left)
	}
	if got := app.findAccount(account.Token); got.Attempt == nil || got.Attempt.Token != recent.Token {
		t.Errorf("the active sign-in was touched: %+v", got.Attempt)
	}
}

func TestFreeingSpaceNeedsTheRunnerPermission(t *testing.T) {
	app := newTestApp(t)

	expectStatus(t, app.do(request{method: "POST", path: runnersCleanupPath, accessToken: app.tokenWith(access.PermissionBotRead)}), http.StatusForbidden)
}

func TestANegativeFreedSizeIsRefused(t *testing.T) {
	app := newTestApp(t)
	runner := app.registerRunner("build-box", heartbeatOptions{tools: []string{"claude"}})

	resp := app.runnerDo(runner.Secret, "POST", "/api/genagents/v1/runner/heartbeat", map[string]any{
		"containerEngine": "docker", "tools": map[string]string{}, "maxConcurrency": 2, "cleanup": map[string]any{"freedBytes": -1},
	})

	expectValidationError(t, resp, "heartbeat")
}
