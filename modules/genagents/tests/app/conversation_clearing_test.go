package app_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/access"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runs"
)

const clearTaskConversationPath = "/api/genagents/v1/tasks/task-1/conversation"

func (app *testApp) countRows(query string) int {
	app.t.Helper()
	var count int
	if err := app.db.QueryRow(context.Background(), query).Scan(&count); err != nil {
		app.t.Fatalf("count: %v", err)
	}
	return count
}

func TestClearingAConversationDeletesItsEntriesAndRuns(t *testing.T) {
	app := newTestApp(t)
	runner, claimed, _ := app.claimedRunOf(runs.RunKindTask, nil)
	app.tasks.SetTask("task-1", nil)
	app.postEvents(runner, claimed.RunToken, eventBody{Sequence: 1, Type: "result", Payload: map[string]any{"summary": "done"}})

	expectStatus(t, app.do(request{method: "DELETE", path: clearTaskConversationPath}), http.StatusNoContent)

	if got := app.readConversation("task-1", ""); len(got.Entries) != 0 || got.Conversation != nil {
		t.Errorf("conversation = %+v", got)
	}
	if entries, runCount, conversations := app.countRows(`SELECT count(*) FROM conversation_entries`), app.countRows(`SELECT count(*) FROM runs`),
		app.countRows(`SELECT count(*) FROM conversations`); entries != 0 || runCount != 0 || conversations != 0 {
		t.Errorf("rows left: entries = %d, runs = %d, conversations = %d", entries, runCount, conversations)
	}
}

func TestClearingStopsTheActiveRunAndIgnoresItsLateEvents(t *testing.T) {
	app := newTestApp(t)
	runner, claimed, _ := app.claimedRunOf(runs.RunKindTask, nil)
	app.tasks.SetTask("task-1", nil)
	app.postEvents(runner, claimed.RunToken, eventBody{Sequence: 1, Type: "assistant_message", Payload: map[string]any{"text": "working"}})

	expectStatus(t, app.do(request{method: "DELETE", path: clearTaskConversationPath}), http.StatusNoContent)

	app.runStateIs(claimed.RunToken, "cancelled")
	late := app.postEvents(runner, claimed.RunToken, eventBody{Sequence: 2, Type: "assistant_message", Payload: map[string]any{"text": "still here"}})
	expectStatus(t, late, http.StatusOK)
	if entries := app.countRows(`SELECT count(*) FROM conversation_entries`); entries != 0 {
		t.Errorf("entries after the clear = %d", entries)
	}
	if got := app.readConversation("task-1", ""); len(got.Entries) != 0 || got.ActiveRun != nil {
		t.Errorf("conversation = %+v", got)
	}
}

func TestClearingATaskWithoutAConversationIsANoOp(t *testing.T) {
	app := newTestApp(t)
	app.tasks.SetTask("task-1", nil)

	expectStatus(t, app.do(request{method: "DELETE", path: clearTaskConversationPath}), http.StatusNoContent)
}

func TestClearingNeedsRunWriteAndTheTask(t *testing.T) {
	app := newTestApp(t)
	app.claimedRunOf(runs.RunKindTask, nil)
	app.tasks.SetTask("task-1", nil)

	expectStatus(t, app.do(request{method: "DELETE", path: clearTaskConversationPath, accessToken: app.tokenWith(access.PermissionRunRead)}), http.StatusForbidden)
	app.tasks.Forbid("task-1")
	expectStatus(t, app.do(request{method: "DELETE", path: clearTaskConversationPath}), http.StatusForbidden)

	if runCount := app.countRows(`SELECT count(*) FROM runs`); runCount != 1 {
		t.Errorf("runs = %d", runCount)
	}
}
