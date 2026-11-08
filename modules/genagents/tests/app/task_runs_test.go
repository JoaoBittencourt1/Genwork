package app_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/access"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runs"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/taskruns"
)

const assignmentsPath = "/api/genagents/v1/task-assignments"

type conversationBody struct {
	Conversation *string `json:"conversation"`
	Entries      []struct {
		Sequence    int64          `json:"sequence"`
		Type        string         `json:"type"`
		Payload     map[string]any `json:"payload"`
		AuthorToken *string        `json:"authorToken"`
	} `json:"entries"`
	LastSequence int64 `json:"lastSequence"`
	ActiveRun    *struct {
		Token   string `json:"token"`
		Kind    string `json:"kind"`
		State   string `json:"state"`
		BotName string `json:"botName"`
	} `json:"activeRun"`
}

func (app *testApp) readConversation(taskToken, query string) conversationBody {
	app.t.Helper()
	resp := app.do(request{method: "GET", path: "/api/genagents/v1/tasks/" + taskToken + "/conversation" + query})
	expectStatus(app.t, resp, http.StatusOK)
	var body conversationBody
	resp.decode(app.t, &body)
	return body
}

func (app *testApp) notifyAssignment(taskToken string) response {
	return app.do(request{method: "POST", path: assignmentsPath, body: map[string]any{"taskToken": taskToken}})
}

// assignedDevBot creates an enabled dev bot, a task on a board with a repository assigned to it, and returns both.
func (app *testApp) assignedDevBot(taskToken string, mutateTask func(map[string]any)) botBody {
	app.t.Helper()
	bot := app.createBotOn(app.claudeAccount().Token, nil)
	app.tasks.SetTask(taskToken, mutateTask)
	member := bot.MemberToken
	app.tasks.Assign(taskToken, &member, "bot", app.clock.Now())
	return bot
}

func (app *testApp) activeRunToken(taskToken string) string {
	app.t.Helper()
	conversation := app.readConversation(taskToken, "")
	if conversation.ActiveRun == nil {
		return ""
	}
	return conversation.ActiveRun.Token
}

func TestAssigningATaskToADevBotQueuesARun(t *testing.T) {
	app := newTestApp(t)
	bot := app.assignedDevBot("task-1", nil)

	expectStatus(t, app.notifyAssignment("task-1"), http.StatusAccepted)

	conversation := app.readConversation("task-1", "")
	if conversation.ActiveRun == nil || conversation.ActiveRun.State != "queued" || conversation.ActiveRun.Kind != "task" || conversation.ActiveRun.BotName != bot.Name {
		t.Fatalf("conversation = %+v", conversation)
	}
	if len(conversation.Entries) != 1 || conversation.Entries[0].Type != "status" || conversation.Entries[0].Payload["code"] != "run_queued" {
		t.Errorf("entries = %+v", conversation.Entries)
	}
	var spec string
	var requester string
	if err := app.db.QueryRow(context.Background(), `SELECT spec::text, requester_user_token FROM runs WHERE token = $1`, conversation.ActiveRun.Token).Scan(&spec, &requester); err != nil {
		t.Fatalf("read run: %v", err)
	}
	if requester != adminUserToken || !strings.Contains(spec, "acme/api") || !strings.Contains(spec, "bot/gen-1-") || !strings.Contains(spec, "Speed up the API") {
		t.Errorf("requester = %s, spec = %s", requester, spec)
	}
}

func TestTaskPromptCarriesTheContext(t *testing.T) {
	app := newTestApp(t)
	app.assignedDevBot("task-1", func(task map[string]any) {
		task["title"], task["description"] = "Speed up the API", "Make /orders faster"
	})
	expectStatus(t, app.notifyAssignment("task-1"), http.StatusAccepted)
	var spec runs.RunSpec
	var raw []byte
	if err := app.db.QueryRow(context.Background(), `SELECT spec FROM runs LIMIT 1`).Scan(&raw); err != nil {
		t.Fatalf("read spec: %v", err)
	}
	if err := jsonUnmarshal(raw, &spec); err != nil {
		t.Fatalf("decode spec: %v", err)
	}

	for _, want := range []string{"GEN-1", "Speed up the API", "Genwork", "acme/api", "Make /orders faster", "backend", "Do not push"} {
		if !strings.Contains(spec.Prompt, want) {
			t.Errorf("the prompt lacks %q:\n%s", want, spec.Prompt)
		}
	}
}

func TestAssigningTwiceDoesNotStartASecondRun(t *testing.T) {
	app := newTestApp(t)
	app.assignedDevBot("task-1", nil)
	expectStatus(t, app.notifyAssignment("task-1"), http.StatusAccepted)
	first := app.activeRunToken("task-1")

	expectStatus(t, app.notifyAssignment("task-1"), http.StatusAccepted)

	if second := app.activeRunToken("task-1"); second != first {
		t.Errorf("run changed from %s to %s", first, second)
	}
	var count int
	if err := app.db.QueryRow(context.Background(), `SELECT count(*) FROM runs`).Scan(&count); err != nil || count != 1 {
		t.Errorf("runs = %d, err = %v", count, err)
	}
}

func TestForgedAssignmentHintStartsNothing(t *testing.T) {
	app := newTestApp(t)
	app.createBotOn(app.claudeAccount().Token, nil)
	app.tasks.SetTask("task-1", nil)
	human := "somebody-else"
	app.tasks.Assign("task-1", &human, "human", app.clock.Now())

	expectStatus(t, app.notifyAssignment("task-1"), http.StatusAccepted)

	var count int
	if err := app.db.QueryRow(context.Background(), `SELECT count(*) FROM runs`).Scan(&count); err != nil || count != 0 {
		t.Errorf("runs = %d, err = %v", count, err)
	}
}

func TestHintForATaskTheCallerCannotSeeIsRelayed(t *testing.T) {
	app := newTestApp(t)
	app.tasks.SetTask("task-1", nil)
	app.tasks.Forbid("task-1")

	expectError(t, app.notifyAssignment("task-1"), http.StatusForbidden, "forbidden")
	expectError(t, app.notifyAssignment("missing"), http.StatusNotFound, "task_not_found")
	expectValidationError(t, app.do(request{method: "POST", path: assignmentsPath, body: map[string]any{}}), "taskToken")
}

func TestReassigningCancelsTheRun(t *testing.T) {
	app := newTestApp(t)
	app.assignedDevBot("task-1", nil)
	expectStatus(t, app.notifyAssignment("task-1"), http.StatusAccepted)
	runToken := app.activeRunToken("task-1")
	human := "somebody-else"
	app.tasks.Assign("task-1", &human, "human", app.clock.Now())

	expectStatus(t, app.notifyAssignment("task-1"), http.StatusAccepted)

	app.runStateIs(runToken, "cancelled")
	conversation := app.readConversation("task-1", "")
	last := conversation.Entries[len(conversation.Entries)-1]
	if last.Payload["code"] != "run_cancelled" || last.Payload["params"].(map[string]any)["cause"] != "reassigned" || conversation.ActiveRun != nil {
		t.Errorf("last entry = %+v", last)
	}
}

func TestUnassigningCancelsTheRun(t *testing.T) {
	app := newTestApp(t)
	app.assignedDevBot("task-1", nil)
	expectStatus(t, app.notifyAssignment("task-1"), http.StatusAccepted)
	runToken := app.activeRunToken("task-1")
	app.tasks.Assign("task-1", nil, "", app.clock.Now())

	expectStatus(t, app.notifyAssignment("task-1"), http.StatusAccepted)

	app.runStateIs(runToken, "cancelled")
}

func TestOnlyDevBotsWorkTasks(t *testing.T) {
	app := newTestApp(t)
	reviewer := app.createBotOn(app.claudeAccount().Token, func(body map[string]any) { body["name"], body["kind"] = "Review Bot", "reviewer" })
	app.tasks.SetTask("task-1", nil)
	member := reviewer.MemberToken
	app.tasks.Assign("task-1", &member, "bot", app.clock.Now())

	expectStatus(t, app.notifyAssignment("task-1"), http.StatusAccepted)

	conversation := app.readConversation("task-1", "")
	if conversation.ActiveRun != nil || len(conversation.Entries) != 1 || conversation.Entries[0].Payload["code"] != "only_dev_bots_work_tasks" {
		t.Errorf("conversation = %+v", conversation)
	}
}

func TestBoardWithoutRepositoryFailsTheRunBeforeAnyRunnerClaimsIt(t *testing.T) {
	app := newTestApp(t)
	app.assignedDevBot("task-1", func(task map[string]any) { task["repository"] = nil })

	expectStatus(t, app.notifyAssignment("task-1"), http.StatusAccepted)

	var state, reason string
	if err := app.db.QueryRow(context.Background(), `SELECT state, failure_reason FROM runs`).Scan(&state, &reason); err != nil {
		t.Fatalf("read run: %v", err)
	}
	conversation := app.readConversation("task-1", "")
	last := conversation.Entries[len(conversation.Entries)-1]
	if state != "failed" || reason != "no_repository" || last.Type != "error" || last.Payload["reason"] != "no_repository" {
		t.Errorf("state = %s (%s), last = %+v", state, reason, last)
	}
	runner := app.registerRunner("build-box", heartbeatOptions{tools: []string{"claude"}, github: true})
	if claimed, _ := app.claim(runner, 1); claimed != nil {
		t.Error("a runner claimed a run that had failed")
	}
}

func TestDisabledBotStartsNoRun(t *testing.T) {
	app := newTestApp(t)
	bot := app.assignedDevBot("task-1", nil)
	expectStatus(t, app.do(request{method: "PATCH", path: "/api/genagents/v1/bots/" + bot.Token, body: map[string]any{"enabled": false}}), http.StatusOK)

	expectStatus(t, app.notifyAssignment("task-1"), http.StatusAccepted)

	if app.activeRunToken("task-1") != "" {
		t.Error("a run started for a disabled bot")
	}
}

func TestConversationReadsAreIncrementalAndNeedTaskAccess(t *testing.T) {
	app := newTestApp(t)
	app.assignedDevBot("task-1", nil)
	expectStatus(t, app.notifyAssignment("task-1"), http.StatusAccepted)
	expectStatus(t, app.do(request{method: "POST", path: "/api/genagents/v1/tasks/task-1/conversation/messages", body: map[string]any{"text": "also add a test"}}), http.StatusAccepted)

	all := app.readConversation("task-1", "")
	later := app.readConversation("task-1", "?after=1")

	if len(all.Entries) < 2 || len(later.Entries) != len(all.Entries)-1 || later.Entries[0].Sequence != 2 || later.LastSequence != all.LastSequence {
		t.Errorf("all = %+v, later = %+v", all, later)
	}
	expectValidationError(t, app.do(request{method: "GET", path: "/api/genagents/v1/tasks/task-1/conversation?after=x"}), "after")
	app.tasks.Forbid("task-1")
	expectError(t, app.do(request{method: "GET", path: "/api/genagents/v1/tasks/task-1/conversation"}), http.StatusForbidden, "forbidden")
	empty := newTestApp(t)
	empty.tasks.SetTask("task-9", nil)
	if body := empty.readConversation("task-9", ""); body.Conversation != nil || len(body.Entries) != 0 || body.ActiveRun != nil {
		t.Errorf("a task without runs = %+v", body)
	}
}

func TestPostingMessagesNeedsRunWriteAndValidText(t *testing.T) {
	app := newTestApp(t)
	app.assignedDevBot("task-1", nil)
	reader := app.tokenWith(access.PermissionRunRead)
	path := "/api/genagents/v1/tasks/task-1/conversation/messages"

	expectError(t, app.do(request{method: "POST", path: path, accessToken: reader, body: map[string]any{"text": "hi"}}), http.StatusForbidden, "forbidden")
	expectValidationError(t, app.do(request{method: "POST", path: path, body: map[string]any{"text": " "}}), "message")
	expectValidationError(t, app.do(request{method: "POST", path: path, body: map[string]any{"text": strings.Repeat("x", 8001)}}), "message")
	expectStatus(t, app.do(request{method: "GET", path: "/api/genagents/v1/tasks/task-1/conversation", accessToken: reader}), http.StatusOK)
	expectError(t, app.do(request{method: "GET", path: "/api/genagents/v1/tasks/task-1/conversation", accessToken: app.tokenWith()}), http.StatusForbidden, "forbidden")
}

func TestMessageToAWorkingBotIsDeliveredToItsRunner(t *testing.T) {
	app := newTestApp(t)
	app.assignedDevBot("task-1", nil)
	expectStatus(t, app.notifyAssignment("task-1"), http.StatusAccepted)
	runner := app.registerRunner("build-box", heartbeatOptions{tools: []string{"claude"}, github: true})
	claimed, _ := app.claim(runner, 1)
	expectStatus(t, app.do(request{method: "POST", path: "/api/genagents/v1/tasks/task-1/conversation/messages", body: map[string]any{"text": "use the existing TaskRepository"}}), http.StatusAccepted)

	var pending struct {
		Messages []struct {
			Text        string  `json:"text"`
			AuthorToken *string `json:"authorToken"`
		} `json:"messages"`
	}
	app.runnerDo(runner.Secret, "GET", "/api/genagents/v1/runner/runs/"+claimed.RunToken+"/messages", nil).decode(t, &pending)

	if len(pending.Messages) != 1 || pending.Messages[0].Text != "use the existing TaskRepository" || *pending.Messages[0].AuthorToken != adminUserToken {
		t.Errorf("pending = %+v", pending)
	}
	if app.activeRunToken("task-1") != claimed.RunToken {
		t.Error("the message started another run")
	}
}

func TestMessageAfterTheRunEndedStartsAFollowUpOnTheSameSession(t *testing.T) {
	app := newTestApp(t)
	app.assignedDevBot("task-1", nil)
	expectStatus(t, app.notifyAssignment("task-1"), http.StatusAccepted)
	runner := app.registerRunner("build-box", heartbeatOptions{tools: []string{"claude"}, github: true})
	claimed, _ := app.claim(runner, 1)
	app.postEvents(runner, claimed.RunToken, eventBody{Sequence: 1, Type: "result", Payload: map[string]any{"summary": "done", "sessionId": "cli-42"}})

	expectStatus(t, app.do(request{method: "POST", path: "/api/genagents/v1/tasks/task-1/conversation/messages", body: map[string]any{"text": "one more thing"}}), http.StatusAccepted)

	followUp := app.activeRunToken("task-1")
	if followUp == "" || followUp == claimed.RunToken {
		t.Fatalf("follow-up run = %q", followUp)
	}
	var session *string
	var spec []byte
	if err := app.db.QueryRow(context.Background(), `SELECT cli_session_id, spec FROM runs WHERE token = $1`, followUp).Scan(&session, &spec); err != nil {
		t.Fatalf("read follow-up: %v", err)
	}
	if session == nil || *session != "cli-42" || !strings.Contains(string(spec), "one more thing") {
		t.Errorf("session = %v, spec = %s", session, spec)
	}
}

func TestStopCancelsTheActiveRun(t *testing.T) {
	app := newTestApp(t)
	app.assignedDevBot("task-1", nil)
	expectStatus(t, app.notifyAssignment("task-1"), http.StatusAccepted)
	runToken := app.activeRunToken("task-1")
	path := "/api/genagents/v1/tasks/task-1/conversation/stop"

	expectStatus(t, app.do(request{method: "POST", path: path}), http.StatusNoContent)

	app.runStateIs(runToken, "cancelled")
	expectError(t, app.do(request{method: "POST", path: path}), http.StatusConflict, "run_not_active")
	expectError(t, app.do(request{method: "POST", path: "/api/genagents/v1/tasks/task-1/conversation/stop", accessToken: app.tokenWith(access.PermissionRunRead)}), http.StatusForbidden, "forbidden")
}

func TestReconciliationStartsRunsForAssignmentsTheHintMissed(t *testing.T) {
	app := newTestApp(t)
	app.assignedDevBot("task-1", nil)

	app.reconcile()

	if app.activeRunToken("task-1") == "" {
		t.Fatal("no run was started")
	}
	first := app.activeRunToken("task-1")
	app.reconcile()
	if app.activeRunToken("task-1") != first {
		t.Error("reconciling again started another run")
	}
}

func TestReconciliationDoesNotRestartARunThatAlreadyFinished(t *testing.T) {
	app := newTestApp(t)
	app.assignedDevBot("task-1", nil)
	app.reconcile()
	runner := app.registerRunner("build-box", heartbeatOptions{tools: []string{"claude"}, github: true})
	claimed, _ := app.claim(runner, 1)
	app.postEvents(runner, claimed.RunToken, eventBody{Sequence: 1, Type: "result", Payload: map[string]any{"summary": "done"}})
	app.clock.Advance(time.Minute)

	app.reconcile()

	if app.activeRunToken("task-1") != "" {
		t.Error("a finished run was started again")
	}
}

func TestReconciliationStartsANewRunForAFreshAssignment(t *testing.T) {
	app := newTestApp(t)
	bot := app.assignedDevBot("task-1", nil)
	app.reconcile()
	runner := app.registerRunner("build-box", heartbeatOptions{tools: []string{"claude"}, github: true})
	claimed, _ := app.claim(runner, 1)
	app.postEvents(runner, claimed.RunToken, eventBody{Sequence: 1, Type: "result", Payload: map[string]any{"summary": "done"}})
	app.clock.Advance(time.Minute)
	member := bot.MemberToken
	app.tasks.Assign("task-1", &member, "bot", app.clock.Now())

	app.reconcile()

	if app.activeRunToken("task-1") == "" {
		t.Error("a new assignment started nothing")
	}
}

func TestReconciliationCancelsRunsForReleasedTasks(t *testing.T) {
	app := newTestApp(t)
	app.assignedDevBot("task-1", nil)
	app.reconcile()
	runToken := app.activeRunToken("task-1")
	human := "somebody-else"
	app.tasks.Assign("task-1", &human, "human", app.clock.Now())

	app.reconcile()

	app.runStateIs(runToken, "cancelled")
}

func TestReconciliationKeepsARunWhoseTaskLeftTheAssignedList(t *testing.T) {
	app := newTestApp(t)
	bot := app.assignedDevBot("task-1", nil)
	app.reconcile()
	runToken := app.activeRunToken("task-1")
	// The assigned list leaves out tasks in a done column, but the bot still holds the task.
	app.tasks.HideFromAssignedList("task-1")

	app.reconcile()

	app.runStateIs(runToken, "queued")
	_ = bot
}

func (app *testApp) reconcile() {
	app.t.Helper()
	workers, err := appWorkers(app)
	if err != nil {
		app.t.Fatalf("workers: %v", err)
	}
	for _, worker := range workers {
		if reconciler, ok := worker.(interface{ ReconcileOnce(context.Context) error }); ok {
			if err := reconciler.ReconcileOnce(context.Background()); err != nil {
				app.t.Fatalf("reconcile: %v", err)
			}
		}
	}
}

func TestReviewRequestsFollowTheRules(t *testing.T) {
	app := newTestApp(t)
	account := app.claudeAccount()
	app.tasks.SetTask("task-1", func(task map[string]any) {
		task["pullRequests"] = []map[string]any{
			{"repository": "acme/api", "number": 12, "title": "GEN-1 Speed up", "state": "open", "url": "https://github.com/acme/api/pull/12"},
			{"repository": "acme/api", "number": 11, "title": "old", "state": "merged", "url": "https://github.com/acme/api/pull/11"},
		}
	})
	path := "/api/genagents/v1/tasks/task-1/bot-reviews"

	expectValidationError(t, app.do(request{method: "POST", path: path, body: map[string]any{}}), "botToken")
	first := app.createBotOn(account.Token, func(body map[string]any) { body["name"], body["kind"] = "Review Bot", "reviewer" })
	resp := app.do(request{method: "POST", path: path, body: map[string]any{}})
	expectStatus(t, resp, http.StatusAccepted)
	conversation := app.readConversation("task-1", "")
	if conversation.ActiveRun == nil || conversation.ActiveRun.Kind != "review" || conversation.ActiveRun.BotName != first.Name {
		t.Fatalf("conversation = %+v", conversation)
	}
	var spec runs.RunSpec
	var raw []byte
	if err := app.db.QueryRow(context.Background(), `SELECT spec FROM runs WHERE kind = 'review'`).Scan(&raw); err != nil {
		t.Fatalf("read spec: %v", err)
	}
	_ = jsonUnmarshal(raw, &spec)
	if len(spec.PullRequests) != 1 || spec.PullRequests[0].Number != 12 || !strings.Contains(spec.Prompt, "acme/api#12") || !strings.Contains(spec.Prompt, `"findings"`) || strings.Contains(spec.Prompt, "#11") {
		t.Errorf("spec = %+v", spec)
	}

	expectError(t, app.do(request{method: "POST", path: path, body: map[string]any{}}), http.StatusConflict, "run_already_active")
}

func TestReviewNeedsAnOpenPullRequest(t *testing.T) {
	app := newTestApp(t)
	app.createBotOn(app.claudeAccount().Token, func(body map[string]any) { body["kind"] = "reviewer" })
	app.tasks.SetTask("task-1", func(task map[string]any) { task["pullRequests"] = []map[string]any{} })
	app.tasks.SetTask("task-2", func(task map[string]any) {
		task["pullRequests"] = []map[string]any{{"repository": "acme/api", "number": 1, "state": "closed", "url": "u"}}
	})

	expectError(t, app.do(request{method: "POST", path: "/api/genagents/v1/tasks/task-1/bot-reviews", body: map[string]any{}}), http.StatusConflict, "no_open_pull_requests")
	expectError(t, app.do(request{method: "POST", path: "/api/genagents/v1/tasks/task-2/bot-reviews", body: map[string]any{}}), http.StatusConflict, "no_open_pull_requests")
}

func TestSeveralReviewerBotsNeedAChoice(t *testing.T) {
	app := newTestApp(t)
	account := app.claudeAccount()
	one := app.createBotOn(account.Token, func(body map[string]any) { body["name"], body["kind"] = "Review One", "reviewer" })
	app.createBotOn(account.Token, func(body map[string]any) { body["name"], body["kind"] = "Review Two", "reviewer" })
	dev := app.createBotOn(account.Token, func(body map[string]any) { body["name"] = "Dev" })
	app.tasks.SetTask("task-1", func(task map[string]any) {
		task["pullRequests"] = []map[string]any{{"repository": "acme/api", "number": 1, "state": "draft", "url": "u"}}
	})
	path := "/api/genagents/v1/tasks/task-1/bot-reviews"

	expectValidationError(t, app.do(request{method: "POST", path: path, body: map[string]any{}}), "botToken")
	expectValidationError(t, app.do(request{method: "POST", path: path, body: map[string]any{"botToken": dev.Token}}), "botToken")
	expectValidationError(t, app.do(request{method: "POST", path: path, body: map[string]any{"botToken": "missing"}}), "botToken")
	expectStatus(t, app.do(request{method: "POST", path: path, body: map[string]any{"botToken": one.Token}}), http.StatusAccepted)
}

func TestReviewRequestsNeedRunWrite(t *testing.T) {
	app := newTestApp(t)

	resp := app.do(request{method: "POST", path: "/api/genagents/v1/tasks/task-1/bot-reviews", accessToken: app.tokenWith(access.PermissionRunRead), body: map[string]any{}})

	expectError(t, resp, http.StatusForbidden, "forbidden")
}

func TestBranchNamesCiteTheTaskKey(t *testing.T) {
	branch := taskruns.BranchName("GEN-42")

	if !strings.HasPrefix(branch, "bot/gen-42-") || len(branch) != len("bot/gen-42-")+6 || strings.ToLower(branch) != branch {
		t.Errorf("branch = %q", branch)
	}
	if branch == taskruns.BranchName("GEN-42") {
		t.Error("two branches for the same task collide")
	}
}

func TestRunListShowsRecentRunsAndFiltersActiveOnes(t *testing.T) {
	app := newTestApp(t)
	app.assignedDevBot("task-1", nil)
	expectStatus(t, app.notifyAssignment("task-1"), http.StatusAccepted)
	runner := app.registerRunner("build-box", heartbeatOptions{tools: []string{"claude"}, github: true})
	claimed, _ := app.claim(runner, 1)
	app.postEvents(runner, claimed.RunToken, eventBody{Sequence: 1, Type: "result", Payload: map[string]any{"summary": "done"}})
	expectStatus(t, app.do(request{method: "POST", path: "/api/genagents/v1/tasks/task-1/conversation/messages", body: map[string]any{"text": "one more"}}), http.StatusAccepted)

	var all, active struct {
		Runs []struct {
			Token     string  `json:"token"`
			Kind      string  `json:"kind"`
			State     string  `json:"state"`
			BotName   string  `json:"botName"`
			TaskToken *string `json:"taskToken"`
			TaskKey   string  `json:"taskKey"`
		} `json:"runs"`
	}
	app.do(request{method: "GET", path: "/api/genagents/v1/runs"}).decode(t, &all)
	app.do(request{method: "GET", path: "/api/genagents/v1/runs?state=active&limit=10"}).decode(t, &active)

	if len(all.Runs) != 2 || all.Runs[0].State != "queued" || all.Runs[1].State != "succeeded" || all.Runs[0].BotName != "Dev Bot" ||
		all.Runs[0].TaskToken == nil || *all.Runs[0].TaskToken != "task-1" || all.Runs[0].TaskKey != "GEN-1" {
		t.Errorf("all runs = %+v", all.Runs)
	}
	if len(active.Runs) != 1 || active.Runs[0].State != "queued" {
		t.Errorf("active runs = %+v", active.Runs)
	}
	expectValidationError(t, app.do(request{method: "GET", path: "/api/genagents/v1/runs?limit=0"}), "limit")
	expectError(t, app.do(request{method: "GET", path: "/api/genagents/v1/runs", accessToken: app.tokenWith()}), http.StatusForbidden, "forbidden")
}

// A review spanning more repositories than gentasks issues one GitHub token for is refused up front, instead of failing
// on the runner without credentials.
func TestReviewAcrossTooManyRepositoriesIsRefused(t *testing.T) {
	app := newTestApp(t)
	app.createBotOn(app.claudeAccount().Token, func(body map[string]any) { body["kind"] = "reviewer" })
	pullRequests := func(repositories int) []map[string]any {
		listed := []map[string]any{}
		for index := 0; index < repositories; index++ {
			repository := fmt.Sprintf("acme/repo-%d", index)
			listed = append(listed, map[string]any{"repository": repository, "number": 1, "state": "open", "url": "https://github.com/" + repository + "/pull/1"})
		}
		return listed
	}
	app.tasks.SetTask("task-1", func(task map[string]any) { task["pullRequests"] = pullRequests(runs.MaxRunRepositories + 1) })
	app.tasks.SetTask("task-2", func(task map[string]any) { task["pullRequests"] = pullRequests(runs.MaxRunRepositories) })

	body := expectError(t, app.do(request{method: "POST", path: "/api/genagents/v1/tasks/task-1/bot-reviews", body: map[string]any{}}), http.StatusConflict, "too_many_review_repositories")

	if body.Error.Fields["max"] != "10" {
		t.Errorf("fields = %v", body.Error.Fields)
	}
	expectStatus(t, app.do(request{method: "POST", path: "/api/genagents/v1/tasks/task-2/bot-reviews", body: map[string]any{}}), http.StatusAccepted)
}
