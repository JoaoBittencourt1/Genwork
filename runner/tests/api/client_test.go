package api_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/api"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/genagentstest"
)

func newClient(t *testing.T) (*api.Client, *genagentstest.FakeGenagents) {
	t.Helper()
	fake := genagentstest.NewFakeGenagents(t)
	return api.NewClient(fake.URL(), genagentstest.Secret, nil), fake
}

func TestClaimReturnsNilWhenThereIsNoWork(t *testing.T) {
	client, _ := newClient(t)

	run, err := client.Claim(context.Background(), 1)

	if err != nil || run != nil {
		t.Errorf("run = %+v, err = %v", run, err)
	}
}

func TestClaimDecodesTheRunSpecification(t *testing.T) {
	client, fake := newClient(t)
	fake.QueueRun(genagentstest.TaskRun("run-1"))

	run, err := client.Claim(context.Background(), 2)

	if err != nil || run == nil {
		t.Fatalf("run = %+v, err = %v", run, err)
	}
	if run.RunToken != "run-1" || run.Kind != "task" || run.Bot.Name != "Dev Bot" || run.Account.APIKey != "sk-ant-run-secret-0001" || run.Repository != "acme/api" ||
		run.Branch != "bot/gen-1-abc123" || len(run.Skills) != 1 || run.TimeoutMinutes != 5 || run.ResumeSession != nil {
		t.Errorf("run = %+v", run)
	}
}

func TestEventsMessagesAndTokens(t *testing.T) {
	client, fake := newClient(t)
	fake.QueueRun(genagentstest.TaskRun("run-1"))
	fake.QueueMessage("run-1", 3, "use the repository")
	ctx := context.Background()

	state, err := client.PostEvents(ctx, "run-1", []api.Event{{Sequence: 1, Type: "status", Payload: []byte(`{"state":"waiting_for_input"}`)}})
	if err != nil || state != "waiting_for_input" {
		t.Errorf("state = %q, err = %v", state, err)
	}
	if polled, err := client.RunState(ctx, "run-1"); err != nil || polled != "waiting_for_input" {
		t.Errorf("polled = %q, err = %v", polled, err)
	}
	messages, err := client.ListMessages(ctx, "run-1")
	if err != nil || len(messages) != 1 || messages[0].Text != "use the repository" {
		t.Fatalf("messages = %+v, err = %v", messages, err)
	}
	if err := client.AcknowledgeMessages(ctx, "run-1", 3); err != nil || fake.Acked["run-1"] != 3 {
		t.Errorf("ack err = %v, acked = %d", err, fake.Acked["run-1"])
	}
	if remaining, _ := client.ListMessages(ctx, "run-1"); len(remaining) != 0 {
		t.Errorf("remaining = %+v", remaining)
	}
	token, err := client.GenworkToken(ctx, "run-1")
	if err != nil || token.AccessToken == "" || token.MCPURL != "http://genwork.test/mcp" || token.ExpiresAt.IsZero() {
		t.Errorf("token = %+v, err = %v", token, err)
	}
	if err := client.ReportAccountStatus(ctx, "acct-1", false, "login expired"); err != nil || len(fake.AccountStatuses) != 1 || fake.AccountStatuses[0]["signedIn"] != false {
		t.Errorf("account status err = %v, recorded = %v", err, fake.AccountStatuses)
	}
}

func TestHeartbeatAnswerListsTheAccountsTheRunnerHolds(t *testing.T) {
	client, fake := newClient(t)
	fake.AssignAccount("acct-1", "anthropic")

	answer, err := client.SendHeartbeat(context.Background(), api.Heartbeat{Tools: map[string]string{"claude": "1.0.0"}, MaxConcurrency: 2})

	if err != nil || len(answer.Accounts) != 1 || answer.Accounts[0].Provider != "anthropic" || fake.HeartbeatCount() != 1 {
		t.Errorf("answer = %+v, err = %v", answer, err)
	}
}

func TestRevokedRunnerGetsAStatusError(t *testing.T) {
	client, fake := newClient(t)
	fake.Revoke()

	_, err := client.SendHeartbeat(context.Background(), api.Heartbeat{})

	var statusErr *api.StatusError
	if !errors.As(err, &statusErr) || !statusErr.IsRevoked() || statusErr.Code != "invalid_runner_secret" {
		t.Errorf("error = %v", err)
	}
	if _, err := client.Claim(context.Background(), 1); err == nil {
		t.Error("a revoked runner claimed")
	}
}

func TestWrongSecretIsRefused(t *testing.T) {
	fake := genagentstest.NewFakeGenagents(t)
	client := api.NewClient(fake.URL(), "wrong", nil)

	_, err := client.RunState(context.Background(), "run-1")

	var statusErr *api.StatusError
	if !errors.As(err, &statusErr) || statusErr.Status != 401 || statusErr.Error() == "" {
		t.Errorf("error = %v", err)
	}
}

func TestSignInsAreListedAndReported(t *testing.T) {
	client, fake := newClient(t)
	fake.SetSignIns(map[string]any{"token": "si-1", "accountToken": "acct-1", "provider": "anthropic", "state": "code_submitted", "code": "pasted", "expiresAt": "2026-11-07T12:10:00Z"})

	listed, err := client.ListSignIns(context.Background())
	if err != nil || len(listed) != 1 || listed[0].Code != "pasted" || listed[0].ExpiresAt.IsZero() {
		t.Fatalf("listed = %+v, err = %v", listed, err)
	}
	err = client.ReportSignInProgress(context.Background(), "si-1", api.SignInProgress{State: "awaiting_user", URL: "https://example.com", ExpectsCode: true})

	reports := fake.Reports()
	if err != nil || len(reports) != 1 || reports[0].Token != "si-1" || reports[0].Payload["url"] != "https://example.com" || reports[0].Payload["expectsCode"] != true {
		t.Errorf("reports = %+v, err = %v", reports, err)
	}
}

func TestGithubTokenIsDecodedOrRefusedWithTheCode(t *testing.T) {
	client, fake := newClient(t)
	fake.SetGithubToken("run-1", "ghs_token")

	token, err := client.GithubToken(context.Background(), "run-1")
	if err != nil || token.Token != "ghs_token" || token.ExpiresAt.IsZero() {
		t.Fatalf("token = %+v, err = %v", token, err)
	}
	_, err = client.GithubToken(context.Background(), "run-2")

	var statusErr *api.StatusError
	if !errors.As(err, &statusErr) || statusErr.Code != "github_token_unavailable" || statusErr.Reason != "github_not_connected" {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(err.Error(), "github_not_connected") {
		t.Errorf("the logged error hides the reason: %v", err)
	}
}
