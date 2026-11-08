package app_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runs"
)

const recordedRepositoryToken = "ghs_recordedInstallationToken0123456789"

func (app *testApp) requestGithubToken(runner testRunner, runToken string) response {
	app.t.Helper()
	return app.runnerDo(runner.Secret, "POST", "/api/genagents/v1/runner/runs/"+runToken+"/github-token", nil)
}

func TestATaskRunGetsAGithubTokenAskedWithTheBotsOwnToken(t *testing.T) {
	app := newTestApp(t)
	runner, claimed, bot := app.claimedRunOf(runs.RunKindTask, nil)

	resp := app.requestGithubToken(runner, claimed.RunToken)

	expectStatus(t, resp, http.StatusOK)
	var body struct {
		Token     string `json:"token"`
		ExpiresAt string `json:"expiresAt"`
	}
	resp.decode(t, &body)
	if body.Token != recordedRepositoryToken || body.ExpiresAt == "" || resp.header("Cache-Control") != "no-store" {
		t.Errorf("response = %+v, cache = %q", body, resp.header("Cache-Control"))
	}
	requests := app.tasks.RepositoryTokenRequests()
	if len(requests) != 1 || len(requests[0].Repositories) != 1 || requests[0].Repositories[0] != "acme/api" {
		t.Fatalf("requests = %+v", requests)
	}
	if !strings.Contains(requests[0].Bearer, bot.MemberToken) {
		t.Errorf("gentasks was not asked with the bot's token: %q", requests[0].Bearer)
	}
}

func TestAChatRunGetsNoGithubToken(t *testing.T) {
	app := newTestApp(t)
	runner, claimed, _ := app.claimedRunOf(runs.RunKindChat, func(body map[string]any) { body["kind"] = "assistant" })

	body := expectError(t, app.requestGithubToken(runner, claimed.RunToken), http.StatusConflict, "github_token_unavailable")

	if body.Error.Fields["reason"] != "not_applicable" || len(app.tasks.RepositoryTokenRequests()) != 0 {
		t.Errorf("fields = %v", body.Error.Fields)
	}
}

func TestGentasksRefusingIsRelayedWithItsReason(t *testing.T) {
	app := newTestApp(t)
	runner, claimed, _ := app.claimedRunOf(runs.RunKindTask, nil)
	app.tasks.RefuseRepositoryTokens("github_app_not_installed")

	body := expectError(t, app.requestGithubToken(runner, claimed.RunToken), http.StatusConflict, "github_token_unavailable")

	if body.Error.Fields["reason"] != "github_app_not_installed" {
		t.Errorf("fields = %v", body.Error.Fields)
	}
}

func TestGentasksBeingDownIsRelayedAsUnavailable(t *testing.T) {
	app := newTestApp(t)
	runner, claimed, _ := app.claimedRunOf(runs.RunKindTask, nil)
	app.tasks.GoDown()

	body := expectError(t, app.requestGithubToken(runner, claimed.RunToken), http.StatusConflict, "github_token_unavailable")

	if body.Error.Fields["reason"] != "gentasks_status_503" {
		t.Errorf("fields = %v", body.Error.Fields)
	}
}

func TestOnlyTheRunnerHoldingTheRunGetsItsGithubToken(t *testing.T) {
	app := newTestApp(t)
	_, claimed, _ := app.claimedRunOf(runs.RunKindTask, nil)
	other := app.registerRunner("other-box", heartbeatOptions{tools: []string{"claude"}})

	resp := app.requestGithubToken(other, claimed.RunToken)

	if resp.status == http.StatusOK || len(app.tasks.RepositoryTokenRequests()) != 0 {
		t.Errorf("another runner got the token: %d %s", resp.status, resp.body)
	}
}

func TestAnEndedRunGetsNoGithubToken(t *testing.T) {
	app := newTestApp(t)
	runner, claimed, _ := app.claimedRunOf(runs.RunKindTask, nil)
	app.postEvents(runner, claimed.RunToken, eventBody{Sequence: 1, Type: "error", Payload: map[string]any{"reason": "github_credentials_missing", "message": "no credentials"}})

	resp := app.requestGithubToken(runner, claimed.RunToken)

	if resp.status == http.StatusOK {
		t.Errorf("an ended run got a token: %s", resp.body)
	}
	if reason := app.runFailureReason(claimed.RunToken); reason != "github_credentials_missing" {
		t.Errorf("failure reason = %q", reason)
	}
}
