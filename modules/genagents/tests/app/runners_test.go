package app_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/access"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runners"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runs"
)

func TestRunnerAdminNeedsThePermission(t *testing.T) {
	app := newTestApp(t)
	member := app.tokenWith(access.PermissionBotRead)

	for _, call := range []request{
		{method: "GET", path: "/api/genagents/v1/runners"},
		{method: "POST", path: "/api/genagents/v1/runners", body: map[string]any{}},
		{method: "DELETE", path: "/api/genagents/v1/runners/x"},
		{method: "POST", path: "/api/genagents/v1/runners/x/removal"},
	} {
		call.accessToken = member
		expectError(t, app.do(call), http.StatusForbidden, "forbidden")
	}
}

func TestCreatingARunnerReturnsTheRegistrationTokenOnce(t *testing.T) {
	app := newTestApp(t)

	created, registrationToken := app.createRunnerViaAPI("build-box")

	if registrationToken == "" || created.Registered || created.Online {
		t.Fatalf("created = %+v", created)
	}
	list := app.do(request{method: "GET", path: "/api/genagents/v1/runners"})
	if strings.Contains(string(list.body), registrationToken) || !strings.Contains(string(list.body), "build-box") {
		t.Errorf("list = %s", list.body)
	}
	var stored []byte
	if err := app.db.QueryRow(context.Background(), `SELECT registration_token_hash FROM runners WHERE token = $1`, created.Token).Scan(&stored); err != nil {
		t.Fatalf("read hash: %v", err)
	}
	if string(stored) == registrationToken || len(stored) != 32 {
		t.Error("the registration token is not stored as a hash")
	}
	expectValidationError(t, app.do(request{method: "POST", path: "/api/genagents/v1/runners", body: map[string]any{"name": " "}}), "name")
	expectError(t, app.do(request{method: "POST", path: "/api/genagents/v1/runners", body: map[string]any{"name": "build-box"}}), http.StatusConflict, "runner_name_taken")
}

func TestRegistrationTokenIsSingleUse(t *testing.T) {
	app := newTestApp(t)
	_, registrationToken := app.createRunnerViaAPI("build-box")
	exchange := func() response {
		return app.do(request{method: "POST", path: "/api/genagents/v1/runner/register", anonymous: true, body: map[string]any{"registrationToken": registrationToken}})
	}

	first, second := exchange(), exchange()

	expectStatus(t, first, http.StatusOK)
	expectError(t, second, http.StatusUnauthorized, "invalid_registration_token")
	var registered struct {
		Secret string `json:"secret"`
	}
	first.decode(t, &registered)
	var stored []byte
	if err := app.db.QueryRow(context.Background(), `SELECT secret_hash FROM runners WHERE name = 'build-box'`).Scan(&stored); err != nil {
		t.Fatalf("read hash: %v", err)
	}
	if string(stored) == registered.Secret || len(stored) != 32 {
		t.Error("the runner secret is not stored as a hash")
	}
}

func TestRegistrationTokenExpiresAfterAnHour(t *testing.T) {
	app := newTestApp(t)
	_, registrationToken := app.createRunnerViaAPI("build-box")

	app.clock.Advance(61 * time.Minute)

	resp := app.do(request{method: "POST", path: "/api/genagents/v1/runner/register", anonymous: true, body: map[string]any{"registrationToken": registrationToken}})
	expectError(t, resp, http.StatusUnauthorized, "invalid_registration_token")
}

func TestRunnerRoutesRejectBadSecrets(t *testing.T) {
	app := newTestApp(t)
	runner := app.registerRunner("build-box", heartbeatOptions{})

	for _, secret := range []string{"", "wrong", runner.Token} {
		expectError(t, app.runnerDo(secret, "POST", "/api/genagents/v1/runner/heartbeat", map[string]any{}), http.StatusUnauthorized, "invalid_runner_secret")
	}
	expectError(t, app.do(request{method: "POST", path: "/api/genagents/v1/runner/claims", anonymous: true, body: map[string]any{}}), http.StatusUnauthorized, "invalid_runner_secret")
}

func TestHeartbeatShowsTheRunnerOnlineUntilItStops(t *testing.T) {
	app := newTestApp(t)
	runner := app.registerRunner("build-box", heartbeatOptions{tools: []string{"claude"}, github: true})

	list := func() runnerBody {
		var body struct {
			Runners []runnerBody `json:"runners"`
		}
		app.do(request{method: "GET", path: "/api/genagents/v1/runners"}).decode(t, &body)
		return body.Runners[0]
	}
	online := list()
	if !online.Online || !online.Registered || !online.HasGithubCredentials || online.Tools["claude"] != "1.0.0" || online.SandboxImage == nil {
		t.Fatalf("runner = %+v", online)
	}

	app.clock.Advance(89 * time.Second)
	if !list().Online {
		t.Error("the runner went offline before 90 seconds")
	}
	app.clock.Advance(2 * time.Second)
	if list().Online {
		t.Error("the runner is still online after 90 seconds")
	}
	expectStatus(t, app.sendHeartbeat(runner, heartbeatOptions{}), http.StatusOK)
	if !list().Online {
		t.Error("a heartbeat did not bring the runner back")
	}
}

func TestHeartbeatValidationAndAccountAssignments(t *testing.T) {
	app := newTestApp(t)
	runner := app.registerRunner("build-box", heartbeatOptions{tools: []string{"claude"}})
	var account accountBody
	app.createAccount(map[string]any{"name": "Claude", "provider": "anthropic", "mode": "subscription", "runnerToken": runner.Token}).decode(t, &account)

	resp := app.sendHeartbeat(runner, heartbeatOptions{tools: []string{"claude"}})

	expectStatus(t, resp, http.StatusOK)
	var body struct {
		Accounts []struct {
			Token    string `json:"token"`
			Provider string `json:"provider"`
		} `json:"accounts"`
	}
	resp.decode(t, &body)
	if len(body.Accounts) != 1 || body.Accounts[0].Token != account.Token || body.Accounts[0].Provider != "anthropic" {
		t.Errorf("accounts = %+v", body.Accounts)
	}
	expectValidationError(t, app.runnerDo(runner.Secret, "POST", "/api/genagents/v1/runner/heartbeat", map[string]any{"maxConcurrency": 0}), "heartbeat")
}

func TestSignInReportMakesTheAccountReadyOrFailed(t *testing.T) {
	app := newTestApp(t)
	runner := app.registerRunner("build-box", heartbeatOptions{tools: []string{"claude"}})
	other := app.registerRunner("other-box", heartbeatOptions{tools: []string{"claude"}})
	var account accountBody
	app.createAccount(map[string]any{"name": "Claude", "provider": "anthropic", "mode": "subscription", "runnerToken": runner.Token}).decode(t, &account)
	path := "/api/genagents/v1/runner/accounts/" + account.Token + "/status"

	expectError(t, app.runnerDo(other.Secret, "POST", path, map[string]any{"signedIn": true}), http.StatusNotFound, "account_not_found")
	expectNoContent(t, app.runnerDo(runner.Secret, "POST", path, map[string]any{"signedIn": true}))
	if got := app.findAccount(account.Token); got.Status != "ready" {
		t.Errorf("status after sign-in = %s", got.Status)
	}
	expectNoContent(t, app.runnerDo(runner.Secret, "POST", path, map[string]any{"signedIn": false, "message": "login expired"}))
	if got := app.findAccount(account.Token); got.Status != "error" || got.LastError == nil || *got.LastError != "login expired" {
		t.Errorf("account after failure = %+v", got)
	}
}

func (app *testApp) findAccount(token string) accountBody {
	app.t.Helper()
	var body struct {
		Accounts []accountBody `json:"accounts"`
	}
	app.do(request{method: "GET", path: "/api/genagents/v1/accounts"}).decode(app.t, &body)
	for _, account := range body.Accounts {
		if account.Token == token {
			return account
		}
	}
	app.t.Fatalf("account %s not found", token)
	return accountBody{}
}

func TestRevokingARunnerRejectsItAndFailsItsRuns(t *testing.T) {
	app := newTestApp(t)
	runner := app.registerRunner("build-box", heartbeatOptions{tools: []string{"claude"}, github: true})
	bot := app.createBotOn(app.claudeAccount().Token, nil)
	run := app.queueRun(bot.Token, runs.RunKindTask, "task-1")
	claimed, _ := app.claim(runner, 1)
	if claimed == nil || claimed.RunToken != run.Token {
		t.Fatalf("claimed = %+v", claimed)
	}

	expectStatus(t, app.do(request{method: "DELETE", path: "/api/genagents/v1/runners/" + runner.Token}), http.StatusNoContent)

	expectError(t, app.runnerDo(runner.Secret, "POST", "/api/genagents/v1/runner/heartbeat", map[string]any{}), http.StatusUnauthorized, "invalid_runner_secret")
	if app.runState(run.Token) != "failed" || app.runFailureReason(run.Token) != "runner_revoked" {
		t.Errorf("run state = %s (%s)", app.runState(run.Token), app.runFailureReason(run.Token))
	}
	expectError(t, app.do(request{method: "DELETE", path: "/api/genagents/v1/runners/" + runner.Token}), http.StatusNotFound, "runner_not_found")
}

func TestRemovingARevokedRunnerDropsItAndKeepsItsRuns(t *testing.T) {
	app := newTestApp(t)
	runner := app.registerRunner("build-box", heartbeatOptions{tools: []string{"claude"}, github: true})
	neverRegistered, _ := app.createRunnerViaAPI("claude")
	bot := app.createBotOn(app.claudeAccount().Token, nil)
	run := app.queueRun(bot.Token, runs.RunKindTask, "task-1")
	claimed, _ := app.claim(runner, 1)
	if claimed == nil || claimed.RunToken != run.Token {
		t.Fatalf("claimed = %+v", claimed)
	}
	expectStatus(t, app.do(request{method: "DELETE", path: "/api/genagents/v1/runners/" + runner.Token}), http.StatusNoContent)
	expectStatus(t, app.do(request{method: "DELETE", path: "/api/genagents/v1/runners/" + neverRegistered.Token}), http.StatusNoContent)

	expectStatus(t, app.removeRunner(runner.Token), http.StatusNoContent)
	expectStatus(t, app.removeRunner(neverRegistered.Token), http.StatusNoContent)

	if names := app.runnerNames(); len(names) != 0 {
		t.Fatalf("runners = %v", names)
	}
	if app.runState(run.Token) != "failed" || app.runFailureReason(run.Token) != "runner_revoked" {
		t.Errorf("run state = %s (%s)", app.runState(run.Token), app.runFailureReason(run.Token))
	}
	var deletedAt *time.Time
	var secretHash []byte
	if err := app.db.QueryRow(context.Background(), `SELECT deleted_at, secret_hash FROM runners WHERE token = $1`, runner.Token).Scan(&deletedAt, &secretHash); err != nil {
		t.Fatalf("read removed runner: %v", err)
	}
	if deletedAt == nil || secretHash != nil {
		t.Errorf("deleted_at = %v, secret still stored = %t", deletedAt, secretHash != nil)
	}
	expectError(t, app.removeRunner(runner.Token), http.StatusNotFound, "runner_not_found")
	expectError(t, app.runnerDo(runner.Secret, "POST", "/api/genagents/v1/runner/heartbeat", map[string]any{}), http.StatusUnauthorized, "invalid_runner_secret")
	if _, registrationToken := app.createRunnerViaAPI("build-box"); registrationToken == "" {
		t.Fatal("the removed runner's name stayed taken")
	}
}

func TestRemovingAnExpiredUnregisteredRunner(t *testing.T) {
	app := newTestApp(t)
	created, registrationToken := app.createRunnerViaAPI("spare-box")
	listed := app.runnerByToken(created.Token)
	if listed.RegistrationExpiresAt == nil || !listed.RegistrationExpiresAt.Equal(app.clock.Now().Add(runners.RegistrationLifetime)) {
		t.Fatalf("registration expiry = %v", listed.RegistrationExpiresAt)
	}
	expectError(t, app.removeRunner(created.Token), http.StatusConflict, "runner_not_removable")

	app.clock.Advance(runners.RegistrationLifetime + time.Minute)

	expectStatus(t, app.removeRunner(created.Token), http.StatusNoContent)
	if names := app.runnerNames(); len(names) != 0 {
		t.Fatalf("runners = %v", names)
	}
	expectError(t, app.do(request{method: "POST", path: "/api/genagents/v1/runner/register", anonymous: true, body: map[string]any{"registrationToken": registrationToken}}), http.StatusUnauthorized, "invalid_registration_token")
	var registrationHash []byte
	if err := app.db.QueryRow(context.Background(), `SELECT registration_token_hash FROM runners WHERE token = $1`, created.Token).Scan(&registrationHash); err != nil {
		t.Fatalf("read registration hash: %v", err)
	}
	if registrationHash != nil {
		t.Error("the registration token hash outlived the runner")
	}
}

func TestARunnerThatCanStillBeUsedCannotBeRemoved(t *testing.T) {
	app := newTestApp(t)
	live := app.registerRunner("build-box", heartbeatOptions{})
	pending, _ := app.createRunnerViaAPI("spare-box")

	for _, token := range []string{live.Token, pending.Token} {
		expectError(t, app.removeRunner(token), http.StatusConflict, "runner_not_removable")
	}
	if names := app.runnerNames(); len(names) != 2 {
		t.Fatalf("runners = %v", names)
	}
}

func TestRemovingARunnerThatHoldsAccountsIsRefused(t *testing.T) {
	app := newTestApp(t)
	runner := app.registerRunner("build-box", heartbeatOptions{tools: []string{"claude"}})
	app.createAccount(map[string]any{"name": "Claude", "provider": "anthropic", "mode": "subscription", "runnerToken": runner.Token})
	expectStatus(t, app.do(request{method: "DELETE", path: "/api/genagents/v1/runners/" + runner.Token}), http.StatusNoContent)

	body := expectError(t, app.removeRunner(runner.Token), http.StatusConflict, "runner_in_use")

	if body.Error.Fields["accounts"] != "Claude" {
		t.Errorf("accounts = %q", body.Error.Fields["accounts"])
	}
	if names := app.runnerNames(); len(names) != 1 || names[0] != "build-box" {
		t.Fatalf("runners = %v", names)
	}
}

func TestRunnerCannotTouchRunsOfAnotherRunner(t *testing.T) {
	app := newTestApp(t)
	holder := app.registerRunner("holder", heartbeatOptions{tools: []string{"claude"}, github: true})
	other := app.registerRunner("other", heartbeatOptions{tools: []string{"claude"}, github: true})
	bot := app.createBotOn(app.claudeAccount().Token, nil)
	app.queueRun(bot.Token, runs.RunKindTask, "task-1")
	claimed, _ := app.claim(holder, 1)

	base := "/api/genagents/v1/runner/runs/" + claimed.RunToken
	expectError(t, app.runnerDo(other.Secret, "GET", base, nil), http.StatusNotFound, "run_not_found")
	expectError(t, app.runnerDo(other.Secret, "GET", base+"/messages", nil), http.StatusNotFound, "run_not_found")
	expectError(t, app.runnerDo(other.Secret, "POST", base+"/messages/ack", map[string]any{"upToSequence": 1}), http.StatusNotFound, "run_not_found")
	expectError(t, app.runnerDo(other.Secret, "POST", base+"/genwork-token", nil), http.StatusNotFound, "run_not_found")
	expectError(t, app.postEvents(other, claimed.RunToken, eventBody{Sequence: 1, Type: "status", Payload: map[string]any{"state": "running"}}), http.StatusNotFound, "run_not_found")
}
