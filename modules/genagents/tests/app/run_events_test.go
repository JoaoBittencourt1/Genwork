package app_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runs"
)

func (app *testApp) claimedRunOf(kind runs.RunKind, botMutate func(map[string]any)) (testRunner, *claimBody, botBody) {
	app.t.Helper()
	runner := app.registerRunner("build-box", heartbeatOptions{tools: []string{"claude"}, github: true})
	bot := app.createBotOn(app.claudeAccount().Token, botMutate)
	app.queueRun(bot.Token, kind, "task-1")
	claimed, _ := app.claim(runner, 1)
	if claimed == nil {
		app.t.Fatal("the run was not claimed")
	}
	return runner, claimed, bot
}

func TestRetriedEventsAppearOnce(t *testing.T) {
	app := newTestApp(t)
	runner, claimed, _ := app.claimedRunOf(runs.RunKindTask, nil)
	event := eventBody{Sequence: 12, Type: "assistant_message", Payload: map[string]any{"text": "working on it"}}

	first := app.postEvents(runner, claimed.RunToken, event)
	second := app.postEvents(runner, claimed.RunToken, event)

	expectStatus(t, first, http.StatusOK)
	expectStatus(t, second, http.StatusOK)
	var body struct {
		Accepted int    `json:"accepted"`
		State    string `json:"state"`
	}
	second.decode(t, &body)
	if body.Accepted != 0 || body.State != "running" {
		t.Errorf("second post = %+v", body)
	}
	count := 0
	for _, entry := range app.conversationEntries(claimed.RunToken) {
		if entry["text"] == "working on it" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("the event appears %d times", count)
	}
}

func TestEventsKeepTheirOrderWhateverTheBatchOrder(t *testing.T) {
	app := newTestApp(t)
	runner, claimed, _ := app.claimedRunOf(runs.RunKindTask, nil)

	resp := app.postEvents(runner, claimed.RunToken,
		eventBody{Sequence: 2, Type: "tool_use", Payload: map[string]any{"tool": "Edit", "summary": "edit main.go"}},
		eventBody{Sequence: 1, Type: "assistant_message", Payload: map[string]any{"text": "first"}})

	expectStatus(t, resp, http.StatusOK)
	entries := app.conversationEntries(claimed.RunToken)
	last := entries[len(entries)-2:]
	if last[0]["_type"] != "assistant_message" || last[1]["_type"] != "tool_use" {
		t.Errorf("entries = %v", entries)
	}
}

func TestEventValidation(t *testing.T) {
	app := newTestApp(t)
	runner, claimed, _ := app.claimedRunOf(runs.RunKindTask, nil)

	for name, events := range map[string][]eventBody{
		"human message": {{Sequence: 1, Type: "human_message", Payload: map[string]any{"text": "x"}}},
		"unknown type":  {{Sequence: 1, Type: "mystery", Payload: map[string]any{}}},
		"bad sequence":  {{Sequence: 0, Type: "status", Payload: map[string]any{}}},
		"empty batch":   {},
	} {
		t.Run(name, func(t *testing.T) {
			expectValidationError(t, app.postEvents(runner, claimed.RunToken, events...), "events")
		})
	}
	tooMany := make([]eventBody, 201)
	for index := range tooMany {
		tooMany[index] = eventBody{Sequence: int64(index + 1), Type: "status", Payload: map[string]any{}}
	}
	expectValidationError(t, app.postEvents(runner, claimed.RunToken, tooMany...), "events")
}

func TestStatusEventsMoveTheRunBetweenRunningAndWaiting(t *testing.T) {
	app := newTestApp(t)
	runner, claimed, _ := app.claimedRunOf(runs.RunKindTask, nil)

	app.postEvents(runner, claimed.RunToken, eventBody{Sequence: 1, Type: "status", Payload: map[string]any{"state": "waiting_for_input", "sessionId": "cli-1"}})
	if app.runState(claimed.RunToken) != "waiting_for_input" {
		t.Fatalf("state = %s", app.runState(claimed.RunToken))
	}
	app.postEvents(runner, claimed.RunToken, eventBody{Sequence: 2, Type: "status", Payload: map[string]any{"state": "running"}})
	if app.runState(claimed.RunToken) != "running" {
		t.Errorf("state = %s", app.runState(claimed.RunToken))
	}
	var session *string
	if err := app.db.QueryRow(context.Background(), `SELECT cli_session_id FROM runs WHERE token = $1`, claimed.RunToken).Scan(&session); err != nil || session == nil || *session != "cli-1" {
		t.Errorf("cli session = %v, err = %v", session, err)
	}
}

func TestResultEventSucceedsTheRun(t *testing.T) {
	app := newTestApp(t)
	runner, claimed, _ := app.claimedRunOf(runs.RunKindTask, nil)

	resp := app.postEvents(runner, claimed.RunToken, eventBody{Sequence: 1, Type: "result", Payload: map[string]any{"summary": "done", "pullRequestUrl": "https://github.com/acme/api/pull/1"}})

	expectStatus(t, resp, http.StatusOK)
	if app.runState(claimed.RunToken) != "succeeded" {
		t.Errorf("state = %s", app.runState(claimed.RunToken))
	}
}

func TestErrorEventFailsTheRunWithItsReason(t *testing.T) {
	app := newTestApp(t)
	runner, claimed, _ := app.claimedRunOf(runs.RunKindTask, nil)

	app.postEvents(runner, claimed.RunToken, eventBody{Sequence: 1, Type: "error", Payload: map[string]any{"reason": "sandbox_limit", "message": "memory"}})

	if app.runState(claimed.RunToken) != "failed" || app.runFailureReason(claimed.RunToken) != "sandbox_limit" {
		t.Errorf("state = %s (%s)", app.runState(claimed.RunToken), app.runFailureReason(claimed.RunToken))
	}
}

func TestUnknownFailureReasonsBecomeAdapterError(t *testing.T) {
	app := newTestApp(t)
	runner, claimed, _ := app.claimedRunOf(runs.RunKindTask, nil)

	app.postEvents(runner, claimed.RunToken, eventBody{Sequence: 1, Type: "error", Payload: map[string]any{"reason": "made_up"}})

	if app.runFailureReason(claimed.RunToken) != "adapter_error" {
		t.Errorf("reason = %s", app.runFailureReason(claimed.RunToken))
	}
}

func TestProviderAuthFailureMarksAnAPIKeyAccountInError(t *testing.T) {
	app := newTestApp(t)
	runner, claimed, _ := app.claimedRunOf(runs.RunKindTask, nil)

	app.postEvents(runner, claimed.RunToken, eventBody{Sequence: 1, Type: "error", Payload: map[string]any{"authFailure": true, "message": "invalid x-api-key"}})

	if app.runFailureReason(claimed.RunToken) != "account_error" {
		t.Errorf("reason = %s", app.runFailureReason(claimed.RunToken))
	}
	account := app.findAccount(claimed.Account.Token)
	if account.Status != "error" || account.LastError == nil || *account.LastError != "invalid x-api-key" {
		t.Errorf("account = %+v", account)
	}
}

func TestEventsAfterTheRunEndedDoNotReviveIt(t *testing.T) {
	app := newTestApp(t)
	runner, claimed, _ := app.claimedRunOf(runs.RunKindTask, nil)
	app.postEvents(runner, claimed.RunToken, eventBody{Sequence: 1, Type: "result", Payload: map[string]any{"summary": "done"}})

	resp := app.postEvents(runner, claimed.RunToken, eventBody{Sequence: 2, Type: "status", Payload: map[string]any{"state": "running"}})

	expectStatus(t, resp, http.StatusOK)
	if app.runState(claimed.RunToken) != "succeeded" {
		t.Errorf("state = %s", app.runState(claimed.RunToken))
	}
}

func TestRunnerLearnsOfACancellationThroughEvents(t *testing.T) {
	app := newTestApp(t)
	runner, claimed, bot := app.claimedRunOf(runs.RunKindTask, nil)
	expectStatus(t, app.do(request{method: "PATCH", path: "/api/genagents/v1/bots/" + bot.Token, body: map[string]any{"enabled": false}}), http.StatusOK)

	resp := app.postEvents(runner, claimed.RunToken, eventBody{Sequence: 1, Type: "assistant_message", Payload: map[string]any{"text": "still going"}})

	var body struct {
		State string `json:"state"`
	}
	resp.decode(t, &body)
	if body.State != "cancelled" {
		t.Errorf("state = %s", body.State)
	}
	var state struct {
		State string `json:"state"`
	}
	app.runnerDo(runner.Secret, "GET", "/api/genagents/v1/runner/runs/"+claimed.RunToken, nil).decode(t, &state)
	if state.State != "cancelled" {
		t.Errorf("polled state = %s", state.State)
	}
}

func TestMessagesAreDeliveredOnceAcknowledged(t *testing.T) {
	app := newTestApp(t)
	runner, claimed, _ := app.claimedRunOf(runs.RunKindTask, nil)
	app.addHumanMessage(claimed.RunToken, "use the existing TaskRepository")
	app.addHumanMessage(claimed.RunToken, "and add a test")
	path := "/api/genagents/v1/runner/runs/" + claimed.RunToken + "/messages"

	var pending struct {
		Messages []struct {
			Sequence int64  `json:"sequence"`
			Text     string `json:"text"`
		} `json:"messages"`
	}
	app.runnerDo(runner.Secret, "GET", path, nil).decode(t, &pending)
	if len(pending.Messages) != 2 || pending.Messages[0].Text != "use the existing TaskRepository" {
		t.Fatalf("pending = %+v", pending)
	}

	expectNoContent(t, app.runnerDo(runner.Secret, "POST", path+"/ack", map[string]any{"upToSequence": pending.Messages[0].Sequence}))

	var remaining struct {
		Messages []struct {
			Text string `json:"text"`
		} `json:"messages"`
	}
	app.runnerDo(runner.Secret, "GET", path, nil).decode(t, &remaining)
	if len(remaining.Messages) != 1 || remaining.Messages[0].Text != "and add a test" {
		t.Errorf("remaining = %+v", remaining)
	}
}

func (app *testApp) addHumanMessage(runToken, text string) {
	app.t.Helper()
	ctx := context.Background()
	run, err := runs.RunRepository{}.FindByToken(ctx, app.db, runToken)
	if err != nil {
		app.t.Fatalf("find run: %v", err)
	}
	author := adminUserToken
	if _, err := (runs.EntryRepository{}).Append(ctx, app.db, runs.NewEntry{
		ConversationID: run.ConversationID, Type: runs.EntryHumanMessage, AuthorToken: &author, Payload: map[string]any{"text": text}, At: app.clock.Now(),
	}); err != nil {
		app.t.Fatalf("add message: %v", err)
	}
}

func TestGenworkTokenForTaskRunsIsTheBotMembers(t *testing.T) {
	app := newTestApp(t)
	runner, claimed, bot := app.claimedRunOf(runs.RunKindTask, nil)

	resp := app.runnerDo(runner.Secret, "POST", "/api/genagents/v1/runner/runs/"+claimed.RunToken+"/genwork-token", nil)

	expectStatus(t, resp, http.StatusOK)
	var token struct {
		AccessToken string    `json:"accessToken"`
		ExpiresAt   time.Time `json:"expiresAt"`
		MCPURL      string    `json:"mcpUrl"`
	}
	resp.decode(t, &token)
	if token.AccessToken == "" || !contains(token.AccessToken, bot.MemberToken) || token.MCPURL != "http://genwork.test/mcp" || token.ExpiresAt.IsZero() {
		t.Errorf("token = %+v", token)
	}
	if resp.header("Cache-Control") != "no-store" {
		t.Errorf("cache control = %q", resp.header("Cache-Control"))
	}
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && (haystack == needle || indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for index := 0; index+len(needle) <= len(haystack); index++ {
		if haystack[index:index+len(needle)] == needle {
			return index
		}
	}
	return -1
}

func TestNoGenworkTokenAfterTheRunEnds(t *testing.T) {
	app := newTestApp(t)
	runner, claimed, _ := app.claimedRunOf(runs.RunKindTask, nil)
	app.postEvents(runner, claimed.RunToken, eventBody{Sequence: 1, Type: "result", Payload: map[string]any{"summary": "done"}})

	resp := app.runnerDo(runner.Secret, "POST", "/api/genagents/v1/runner/runs/"+claimed.RunToken+"/genwork-token", nil)

	expectError(t, resp, http.StatusConflict, "run_not_active")
}

func TestNoGenworkTokenWhenCoreRefusesTheBot(t *testing.T) {
	app := newTestApp(t)
	runner, claimed, bot := app.claimedRunOf(runs.RunKindTask, nil)
	expectStatus(t, app.do(request{method: "PATCH", path: "/api/genagents/v1/bots/" + bot.Token, body: map[string]any{"enabled": false}}), http.StatusOK)
	app.runStateIs(claimed.RunToken, "cancelled")

	resp := app.runnerDo(runner.Secret, "POST", "/api/genagents/v1/runner/runs/"+claimed.RunToken+"/genwork-token", nil)

	expectError(t, resp, http.StatusConflict, "run_not_active")
}

func (app *testApp) runStateIs(runToken, want string) {
	app.t.Helper()
	if got := app.runState(runToken); got != want {
		app.t.Fatalf("run state = %s, want %s", got, want)
	}
}
