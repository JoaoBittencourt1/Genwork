package app_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/access"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runs"
)

const chatsPath = "/api/genagents/v1/admin-chats"

type chatBody struct {
	Token        string `json:"token"`
	Title        string `json:"title"`
	BotName      string `json:"botName"`
	LastSequence int64  `json:"lastSequence"`
	Entries      []struct {
		Type    string         `json:"type"`
		Payload map[string]any `json:"payload"`
	} `json:"entries"`
	ActiveRun *struct {
		Token string `json:"token"`
		Kind  string `json:"kind"`
		State string `json:"state"`
	} `json:"activeRun"`
}

func (app *testApp) assistantBot() botBody {
	app.t.Helper()
	return app.createBotOn(app.claudeAccount().Token, func(body map[string]any) { body["name"], body["kind"] = "Ops Assistant", "assistant" })
}

func (app *testApp) startChat(botToken string) chatBody {
	app.t.Helper()
	resp := app.do(request{method: "POST", path: chatsPath, body: map[string]any{"botToken": botToken, "title": "Ops"}})
	expectStatus(app.t, resp, http.StatusCreated)
	var chat chatBody
	resp.decode(app.t, &chat)
	return chat
}

func (app *testApp) readChat(token string) chatBody {
	app.t.Helper()
	resp := app.do(request{method: "GET", path: chatsPath + "/" + token})
	expectStatus(app.t, resp, http.StatusOK)
	var chat chatBody
	resp.decode(app.t, &chat)
	return chat
}

func (app *testApp) sendChatMessage(token, text string) response {
	return app.do(request{method: "POST", path: chatsPath + "/" + token + "/messages", body: map[string]any{"text": text}})
}

func (app *testApp) otherAdminToken() string {
	return app.core.MintAccessToken(app.t, "other-admin-token", access.Permissions...)
}

func TestEveryAdminChatEndpointNeedsTheChatPermission(t *testing.T) {
	app := newTestApp(t)
	member := app.tokenWith(access.PermissionBotRead, access.PermissionRunRead, access.PermissionRunWrite)

	for _, call := range []request{
		{method: "GET", path: chatsPath},
		{method: "POST", path: chatsPath, body: map[string]any{}},
		{method: "GET", path: chatsPath + "/x"},
		{method: "PATCH", path: chatsPath + "/x", body: map[string]any{}},
		{method: "DELETE", path: chatsPath + "/x"},
		{method: "POST", path: chatsPath + "/x/messages", body: map[string]any{}},
		{method: "POST", path: chatsPath + "/x/stop"},
		{method: "POST", path: chatsPath + "/x/reopen"},
	} {
		call.accessToken = member
		expectError(t, app.do(call), http.StatusForbidden, "forbidden")
	}
}

func TestStartingAChatGrantsADelegatedSessionAndKeepsItEncrypted(t *testing.T) {
	app := newTestApp(t)
	bot := app.assistantBot()

	chat := app.startChat(bot.Token)

	if chat.Title != "Ops" || chat.Token == "" {
		t.Fatalf("chat = %+v", chat)
	}
	granted := 0
	for _, call := range app.coreAPI.Calls() {
		if call == "POST /api/core/v1/delegated-sessions" {
			granted++
		}
	}
	if granted != 1 {
		t.Errorf("delegated sessions granted = %d", granted)
	}
	var ciphertext []byte
	var owner string
	if err := app.db.QueryRow(context.Background(), `SELECT delegated_refresh_ciphertext, owner_user_token FROM conversations WHERE token = $1`, chat.Token).Scan(&ciphertext, &owner); err != nil {
		t.Fatalf("read chat: %v", err)
	}
	if len(ciphertext) == 0 || strings.Contains(string(ciphertext), "refresh-") || owner != adminUserToken {
		t.Errorf("owner = %s, ciphertext = %q", owner, ciphertext)
	}
}

func TestOnlyEnabledAssistantBotsOpenChats(t *testing.T) {
	app := newTestApp(t)
	account := app.claudeAccount()
	dev := app.createBotOn(account.Token, nil)
	disabled := app.createBotOn(account.Token, func(body map[string]any) { body["name"], body["kind"] = "Off", "assistant" })
	expectStatus(t, app.do(request{method: "PATCH", path: "/api/genagents/v1/bots/" + disabled.Token, body: map[string]any{"enabled": false}}), http.StatusOK)

	for _, botToken := range []string{dev.Token, disabled.Token, "missing"} {
		expectValidationError(t, app.do(request{method: "POST", path: chatsPath, body: map[string]any{"botToken": botToken}}), "botToken")
	}
	expectValidationError(t, app.do(request{method: "POST", path: chatsPath, body: map[string]any{"botToken": dev.Token, "title": strings.Repeat("x", 101)}}), "title")
}

func TestCoreRefusalLeavesNoChat(t *testing.T) {
	app := newTestApp(t)
	bot := app.assistantBot()
	app.coreAPI.RefuseNext(http.StatusForbidden, "forbidden")

	expectError(t, app.do(request{method: "POST", path: chatsPath, body: map[string]any{"botToken": bot.Token}}), http.StatusForbidden, "forbidden")

	var list struct {
		Chats []chatBody `json:"chats"`
	}
	app.do(request{method: "GET", path: chatsPath}).decode(t, &list)
	if len(list.Chats) != 0 {
		t.Errorf("chats = %+v", list.Chats)
	}
}

func TestChatsBelongToTheirAdmin(t *testing.T) {
	app := newTestApp(t)
	chat := app.startChat(app.assistantBot().Token)
	other := app.otherAdminToken()

	for _, call := range []request{
		{method: "GET", path: chatsPath + "/" + chat.Token},
		{method: "PATCH", path: chatsPath + "/" + chat.Token, body: map[string]any{"title": "Mine now"}},
		{method: "DELETE", path: chatsPath + "/" + chat.Token},
		{method: "POST", path: chatsPath + "/" + chat.Token + "/messages", body: map[string]any{"text": "hi"}},
		{method: "POST", path: chatsPath + "/" + chat.Token + "/stop"},
		{method: "POST", path: chatsPath + "/" + chat.Token + "/reopen"},
	} {
		call.accessToken = other
		expectError(t, app.do(call), http.StatusNotFound, "chat_not_found")
	}
	var list struct {
		Chats []chatBody `json:"chats"`
	}
	app.do(request{method: "GET", path: chatsPath, accessToken: other}).decode(t, &list)
	if len(list.Chats) != 0 {
		t.Errorf("another admin lists %+v", list.Chats)
	}
	expectError(t, app.do(request{method: "GET", path: chatsPath + "/unknown"}), http.StatusNotFound, "chat_not_found")
}

func TestTaskConversationsAreNotAdminChats(t *testing.T) {
	app := newTestApp(t)
	app.assignedDevBot("task-1", nil)
	expectStatus(t, app.notifyAssignment("task-1"), http.StatusAccepted)
	conversation := app.readConversation("task-1", "")

	expectError(t, app.do(request{method: "GET", path: chatsPath + "/" + *conversation.Conversation}), http.StatusNotFound, "chat_not_found")
}

func TestEachMessageRunsAsATurnOnTheSameCLISession(t *testing.T) {
	app := newTestApp(t)
	chat := app.startChat(app.assistantBot().Token)
	runner := app.registerRunner("build-box", heartbeatOptions{tools: []string{"claude"}})

	expectStatus(t, app.sendChatMessage(chat.Token, "list my tasks"), http.StatusAccepted)
	first, _ := app.claim(runner, 1)
	if first == nil || first.Kind != "chat" || first.Prompt != "list my tasks" {
		t.Fatalf("first turn = %+v", first)
	}
	expectStatus(t, app.sendChatMessage(chat.Token, "and the host CPU"), http.StatusAccepted)
	if again, _ := app.claim(runner, 1); again != nil {
		t.Fatal("a second turn started while the first was running")
	}
	app.postEvents(runner, first.RunToken, eventBody{Sequence: 1, Type: "result", Payload: map[string]any{"summary": "3 tasks", "sessionId": "cli-chat-1"}})
	expectStatus(t, app.sendChatMessage(chat.Token, "thanks, now archive them"), http.StatusAccepted)

	second, _ := app.claim(runner, 1)
	if second == nil || second.Prompt != "thanks, now archive them" {
		t.Fatalf("second turn = %+v", second)
	}
	var session *string
	if err := app.db.QueryRow(context.Background(), `SELECT cli_session_id FROM runs WHERE token = $1`, second.RunToken).Scan(&session); err != nil || session == nil || *session != "cli-chat-1" {
		t.Errorf("session = %v, err = %v", session, err)
	}
	read := app.readChat(chat.Token)
	humans := 0
	for _, entry := range read.Entries {
		if entry.Type == "human_message" {
			humans++
		}
	}
	if humans != 3 || read.BotName != "Ops Assistant" || read.ActiveRun == nil || read.ActiveRun.Kind != "chat" {
		t.Errorf("chat = %+v", read)
	}
}

func TestChatRunsGetTheAdminsDelegatedToken(t *testing.T) {
	app := newTestApp(t)
	chat := app.startChat(app.assistantBot().Token)
	runner := app.registerRunner("build-box", heartbeatOptions{tools: []string{"claude"}})
	expectStatus(t, app.sendChatMessage(chat.Token, "hi"), http.StatusAccepted)
	claimed, _ := app.claim(runner, 1)

	resp := app.runnerDo(runner.Secret, "POST", "/api/genagents/v1/runner/runs/"+claimed.RunToken+"/genwork-token", nil)

	expectStatus(t, resp, http.StatusOK)
	var token struct {
		AccessToken string `json:"accessToken"`
	}
	resp.decode(t, &token)
	if !strings.HasPrefix(token.AccessToken, "delegated-access-") {
		t.Errorf("token = %s", token.AccessToken)
	}
	again := app.runnerDo(runner.Secret, "POST", "/api/genagents/v1/runner/runs/"+claimed.RunToken+"/genwork-token", nil)
	expectStatus(t, again, http.StatusOK)
}

func TestEndedSessionFailsTheNextTurnUntilTheChatIsReopened(t *testing.T) {
	app := newTestApp(t)
	chat := app.startChat(app.assistantBot().Token)
	runner := app.registerRunner("build-box", heartbeatOptions{tools: []string{"claude"}})
	app.coreAPI.RevokeDelegatedSessions()
	expectStatus(t, app.sendChatMessage(chat.Token, "hi"), http.StatusAccepted)
	claimed, _ := app.claim(runner, 1)

	resp := app.runnerDo(runner.Secret, "POST", "/api/genagents/v1/runner/runs/"+claimed.RunToken+"/genwork-token", nil)

	expectError(t, resp, http.StatusConflict, "session_revoked")
	if app.runState(claimed.RunToken) != "failed" || app.runFailureReason(claimed.RunToken) != "session_revoked" {
		t.Errorf("state = %s (%s)", app.runState(claimed.RunToken), app.runFailureReason(claimed.RunToken))
	}
	entries := app.readChat(chat.Token).Entries
	if last := entries[len(entries)-1]; last.Type != "error" || last.Payload["reason"] != "session_revoked" {
		t.Errorf("last entry = %+v", last)
	}

	expectStatus(t, app.do(request{method: "POST", path: chatsPath + "/" + chat.Token + "/reopen"}), http.StatusNoContent)
	expectStatus(t, app.sendChatMessage(chat.Token, "hi again"), http.StatusAccepted)
	next, _ := app.claim(runner, 1)
	if next == nil {
		t.Fatal("no turn after reopening")
	}
	expectStatus(t, app.runnerDo(runner.Secret, "POST", "/api/genagents/v1/runner/runs/"+next.RunToken+"/genwork-token", nil), http.StatusOK)
}

func TestRenameListAndDeleteChats(t *testing.T) {
	app := newTestApp(t)
	chat := app.startChat(app.assistantBot().Token)

	expectStatus(t, app.do(request{method: "PATCH", path: chatsPath + "/" + chat.Token, body: map[string]any{"title": "Renamed"}}), http.StatusNoContent)
	expectValidationError(t, app.do(request{method: "PATCH", path: chatsPath + "/" + chat.Token, body: map[string]any{"title": strings.Repeat("x", 101)}}), "title")
	var list struct {
		Chats []chatBody `json:"chats"`
	}
	app.do(request{method: "GET", path: chatsPath}).decode(t, &list)
	if len(list.Chats) != 1 || list.Chats[0].Title != "Renamed" {
		t.Fatalf("chats = %+v", list.Chats)
	}

	expectStatus(t, app.do(request{method: "DELETE", path: chatsPath + "/" + chat.Token}), http.StatusNoContent)

	expectError(t, app.do(request{method: "GET", path: chatsPath + "/" + chat.Token}), http.StatusNotFound, "chat_not_found")
	var ciphertext []byte
	if err := app.db.QueryRow(context.Background(), `SELECT delegated_refresh_ciphertext FROM conversations WHERE token = $1`, chat.Token).Scan(&ciphertext); err != nil {
		t.Fatalf("read chat: %v", err)
	}
	if ciphertext != nil {
		t.Error("a deleted chat kept its refresh token")
	}
	logouts := 0
	for _, call := range app.coreAPI.Calls() {
		if call == "POST /api/core/v1/auth/logout" {
			logouts++
		}
	}
	if logouts != 1 {
		t.Errorf("sessions revoked in core = %d", logouts)
	}
}

func TestStoppingAChatCancelsItsRunAndMessagesNeedValidText(t *testing.T) {
	app := newTestApp(t)
	chat := app.startChat(app.assistantBot().Token)
	expectStatus(t, app.sendChatMessage(chat.Token, "hi"), http.StatusAccepted)
	runToken := app.readChat(chat.Token).ActiveRun.Token

	expectStatus(t, app.do(request{method: "POST", path: chatsPath + "/" + chat.Token + "/stop"}), http.StatusNoContent)

	app.runStateIs(runToken, string(runs.RunStateCancelled))
	expectError(t, app.do(request{method: "POST", path: chatsPath + "/" + chat.Token + "/stop"}), http.StatusConflict, "run_not_active")
	expectValidationError(t, app.sendChatMessage(chat.Token, " "), "message")
	expectValidationError(t, app.sendChatMessage(chat.Token, strings.Repeat("x", 8001)), "message")
	expectValidationError(t, app.do(request{method: "GET", path: chatsPath + "/" + chat.Token + "?after=x"}), "after")
}
