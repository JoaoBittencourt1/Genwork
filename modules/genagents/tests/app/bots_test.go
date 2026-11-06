package app_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/access"
)

type botBody struct {
	Token          string  `json:"token"`
	Name           string  `json:"name"`
	Kind           string  `json:"kind"`
	Provider       string  `json:"provider"`
	Model          string  `json:"model"`
	MemberToken    string  `json:"memberToken"`
	Enabled        bool    `json:"enabled"`
	Effort         string  `json:"effort"`
	MaxTurns       int     `json:"maxTurns"`
	TimeoutMinutes int     `json:"timeoutMinutes"`
	Instructions   *string `json:"instructions"`
	Account        struct {
		Token string `json:"token"`
		Name  string `json:"name"`
	} `json:"account"`
	Skills []struct {
		Token string `json:"token"`
		Name  string `json:"name"`
	} `json:"skills"`
	MCPServers []struct {
		Token string `json:"token"`
		Name  string `json:"name"`
	} `json:"mcpServers"`
}

func (app *testApp) createBotRequest(body map[string]any) response {
	return app.do(request{method: "POST", path: "/api/genagents/v1/bots", body: body})
}

func devBotBody(accountToken string) map[string]any {
	return map[string]any{
		"name": "Dev Bot", "kind": "dev", "accountToken": accountToken, "model": "claude-sonnet-5-5", "role": "role-member-token",
	}
}

func (app *testApp) createBotOn(accountToken string, mutate func(map[string]any)) botBody {
	app.t.Helper()
	body := devBotBody(accountToken)
	if mutate != nil {
		mutate(body)
	}
	resp := app.createBotRequest(body)
	expectStatus(app.t, resp, http.StatusCreated)
	var bot botBody
	resp.decode(app.t, &bot)
	return bot
}

func (app *testApp) claudeAccount() accountBody {
	return app.createAPIKeyAccount("Claude — Team", "anthropic", "sk-ant-team-0001")
}

func TestCreateDevBotCreatesItsCoreMember(t *testing.T) {
	app := newTestApp(t)
	account := app.claudeAccount()

	bot := app.createBotOn(account.Token, nil)

	if bot.Kind != "dev" || bot.Provider != "anthropic" || bot.Model != "claude-sonnet-5-5" || bot.Account.Name != "Claude — Team" ||
		!bot.Enabled || bot.Effort != "default" || bot.MaxTurns != 100 || bot.TimeoutMinutes != 60 || bot.MemberToken == "" {
		t.Fatalf("bot = %+v", bot)
	}
	member, found := app.coreAPI.Member(bot.MemberToken)
	if !found || member.Name != "Dev Bot" || member.Role != "role-member-token" || !member.Active {
		t.Errorf("core member = %+v", member)
	}
}

func TestBotSecretIsStoredEncryptedAndNeverReturned(t *testing.T) {
	app := newTestApp(t)
	bot := app.createBotOn(app.claudeAccount().Token, nil)
	member, _ := app.coreAPI.Member(bot.MemberToken)

	var stored []byte
	if err := app.db.QueryRow(context.Background(), `SELECT member_secret_ciphertext FROM bots WHERE token = $1`, bot.Token).Scan(&stored); err != nil {
		t.Fatalf("read stored secret: %v", err)
	}

	if len(stored) == 0 || strings.Contains(string(stored), member.Secret) {
		t.Error("the member secret is not stored encrypted")
	}
	for _, path := range []string{"/api/genagents/v1/bots", "/api/genagents/v1/bots/" + bot.Token} {
		if resp := app.do(request{method: "GET", path: path}); strings.Contains(string(resp.body), member.Secret) || strings.Contains(string(resp.body), "secret") {
			t.Errorf("%s exposes the secret: %s", path, resp.body)
		}
	}
}

func TestBotModelMustBelongToTheAccountProvider(t *testing.T) {
	app := newTestApp(t)
	account := app.claudeAccount()

	resp := app.createBotRequest(func() map[string]any { body := devBotBody(account.Token); body["model"] = "gpt-5"; return body }())

	expectValidationError(t, resp, "model")
	if len(app.coreAPI.Calls()) != 0 {
		t.Errorf("core was called for an invalid bot: %v", app.coreAPI.Calls())
	}
}

func TestBotCanUseACustomModelOfItsAccount(t *testing.T) {
	app := newTestApp(t)
	account := app.claudeAccount()
	expectStatus(t, app.do(request{method: "POST", path: "/api/genagents/v1/accounts/" + account.Token + "/models", body: map[string]any{"modelId": "claude-opus-5-5-next"}}), http.StatusCreated)

	bot := app.createBotOn(account.Token, func(body map[string]any) { body["model"] = "claude-opus-5-5-next" })

	if bot.Model != "claude-opus-5-5-next" {
		t.Errorf("model = %s", bot.Model)
	}
}

func TestBotCreationValidation(t *testing.T) {
	app := newTestApp(t)
	account := app.claudeAccount()
	cases := map[string]func(map[string]any){
		"name":           func(body map[string]any) { body["name"] = " " },
		"kind":           func(body map[string]any) { body["kind"] = "boss" },
		"accountToken":   func(body map[string]any) { body["accountToken"] = "missing" },
		"role":           func(body map[string]any) { body["role"] = "" },
		"effort":         func(body map[string]any) { body["effort"] = "extreme" },
		"maxTurns":       func(body map[string]any) { body["maxTurns"] = 501 },
		"timeoutMinutes": func(body map[string]any) { body["timeoutMinutes"] = 4 },
		"skillTokens":    func(body map[string]any) { body["skillTokens"] = []string{"nope"} },
		"mcpServerTokens": func(body map[string]any) {
			body["mcpServerTokens"] = []string{"nope"}
		},
	}
	for field, mutate := range cases {
		t.Run(field, func(t *testing.T) {
			body := devBotBody(account.Token)
			mutate(body)
			expectValidationError(t, app.createBotRequest(body), field)
		})
	}
	members := 0
	for _, call := range app.coreAPI.Calls() {
		if call == "POST /api/core/v1/bots" {
			members++
		}
	}
	removed := 0
	for _, call := range app.coreAPI.Calls() {
		if strings.HasPrefix(call, "DELETE /api/core/v1/bots/") {
			removed++
		}
	}
	if members != removed {
		t.Errorf("members created %d, removed again %d: a failed bot left a core member behind", members, removed)
	}
}

func TestCoreRefusalLeavesNoBot(t *testing.T) {
	app := newTestApp(t)
	account := app.claudeAccount()
	app.coreAPI.RefuseNext(http.StatusForbidden, "forbidden")

	resp := app.createBotRequest(devBotBody(account.Token))

	expectError(t, resp, http.StatusForbidden, "forbidden")
	list := app.do(request{method: "GET", path: "/api/genagents/v1/bots"})
	if strings.Contains(string(list.body), "Dev Bot") {
		t.Errorf("a refused bot was stored: %s", list.body)
	}
}

func TestBotNamesAreUniqueAndTheMemberIsRemovedAgain(t *testing.T) {
	app := newTestApp(t)
	account := app.claudeAccount()
	app.createBotOn(account.Token, nil)

	resp := app.createBotRequest(devBotBody(account.Token))

	expectError(t, resp, http.StatusConflict, "bot_name_taken")
	if _, found := app.coreAPI.MemberByName("Dev Bot"); !found {
		t.Fatal("the first bot's member disappeared")
	}
	members := 0
	for _, call := range app.coreAPI.Calls() {
		if call == "POST /api/core/v1/bots" {
			members++
		}
	}
	if members != 2 {
		t.Errorf("core member creations = %d", members)
	}
}

func TestBotWithSkillsAndServers(t *testing.T) {
	app := newTestApp(t)
	account := app.claudeAccount()
	skill := app.createGoSkill()
	server := app.createSentry()

	bot := app.createBotOn(account.Token, func(body map[string]any) {
		body["skillTokens"], body["mcpServerTokens"] = []string{skill.Token}, []string{server.Token}
	})

	if len(bot.Skills) != 1 || bot.Skills[0].Name != "go-conventions" || len(bot.MCPServers) != 1 || bot.MCPServers[0].Name != "sentry" {
		t.Fatalf("bot = %+v", bot)
	}
	expectStatus(t, app.do(request{method: "DELETE", path: "/api/genagents/v1/skills/" + skill.Token}), http.StatusNoContent)
	expectStatus(t, app.do(request{method: "DELETE", path: "/api/genagents/v1/mcp-servers/" + server.Token}), http.StatusNoContent)
	var detached botBody
	app.do(request{method: "GET", path: "/api/genagents/v1/bots/" + bot.Token}).decode(t, &detached)
	if len(detached.Skills) != 0 || len(detached.MCPServers) != 0 {
		t.Errorf("deleted skill and server stayed attached: %+v", detached)
	}
}

func TestPatchBotSingleFieldLeavesEverythingElse(t *testing.T) {
	app := newTestApp(t)
	account := app.claudeAccount()
	skill := app.createGoSkill()
	instructions := "Always write tests"
	bot := app.createBotOn(account.Token, func(body map[string]any) {
		body["skillTokens"], body["instructions"] = []string{skill.Token}, instructions
		body["effort"], body["maxTurns"], body["timeoutMinutes"] = "high", 200, 90
	})
	path := "/api/genagents/v1/bots/" + bot.Token

	resp := app.do(request{method: "PATCH", path: path, body: map[string]any{"model": "claude-opus-5-5"}})

	expectStatus(t, resp, http.StatusOK)
	var updated botBody
	resp.decode(t, &updated)
	if updated.Model != "claude-opus-5-5" || updated.Name != bot.Name || updated.Kind != "dev" || updated.Effort != "high" ||
		updated.MaxTurns != 200 || updated.TimeoutMinutes != 90 || updated.Instructions == nil || *updated.Instructions != instructions ||
		len(updated.Skills) != 1 || updated.Account.Token != account.Token || !updated.Enabled {
		t.Errorf("updated = %+v", updated)
	}
	cleared := app.do(request{method: "PATCH", path: path, body: map[string]any{"instructions": nil}})
	var afterClear botBody
	cleared.decode(t, &afterClear)
	if afterClear.Instructions != nil || afterClear.Model != "claude-opus-5-5" {
		t.Errorf("after clearing instructions = %+v", afterClear)
	}
	for _, field := range []string{"name", "kind", "accountToken", "model", "role", "effort", "maxTurns", "timeoutMinutes", "skillTokens", "mcpServerTokens", "enabled"} {
		expectValidationError(t, app.do(request{method: "PATCH", path: path, body: map[string]any{field: nil}}), field)
	}
	expectValidationError(t, app.do(request{method: "PATCH", path: path, body: map[string]any{"model": "gpt-5"}}), "model")
	expectValidationError(t, app.do(request{method: "PATCH", path: path, body: map[string]any{"maxTurns": 0}}), "maxTurns")
}

func TestRenamingAndChangingTheRoleReachTheMember(t *testing.T) {
	app := newTestApp(t)
	bot := app.createBotOn(app.claudeAccount().Token, nil)

	resp := app.do(request{method: "PATCH", path: "/api/genagents/v1/bots/" + bot.Token, body: map[string]any{"name": "Dev Bot 2", "role": "role-admin-token"}})

	expectStatus(t, resp, http.StatusOK)
	member, _ := app.coreAPI.Member(bot.MemberToken)
	if member.Name != "Dev Bot 2" || member.Role != "role-admin-token" {
		t.Errorf("core member = %+v", member)
	}
}

func TestCoreRefusalRollsTheBotChangeBack(t *testing.T) {
	app := newTestApp(t)
	bot := app.createBotOn(app.claudeAccount().Token, nil)
	app.coreAPI.RefuseNext(http.StatusForbidden, "forbidden")

	resp := app.do(request{method: "PATCH", path: "/api/genagents/v1/bots/" + bot.Token, body: map[string]any{"name": "Renamed", "model": "claude-opus-5-5"}})

	expectError(t, resp, http.StatusForbidden, "forbidden")
	var unchanged botBody
	app.do(request{method: "GET", path: "/api/genagents/v1/bots/" + bot.Token}).decode(t, &unchanged)
	if unchanged.Name != "Dev Bot" || unchanged.Model != "claude-sonnet-5-5" {
		t.Errorf("bot changed despite the refusal: %+v", unchanged)
	}
}

func TestDisablingABotDeactivatesItsMember(t *testing.T) {
	app := newTestApp(t)
	bot := app.createBotOn(app.claudeAccount().Token, nil)
	path := "/api/genagents/v1/bots/" + bot.Token

	disabled := app.do(request{method: "PATCH", path: path, body: map[string]any{"enabled": false}})

	expectStatus(t, disabled, http.StatusOK)
	var body botBody
	disabled.decode(t, &body)
	if body.Enabled {
		t.Error("the bot is still enabled")
	}
	if member, _ := app.coreAPI.Member(bot.MemberToken); member.Active {
		t.Error("the member is still active")
	}
	app.do(request{method: "PATCH", path: path, body: map[string]any{"enabled": true}}).decode(t, &body)
	if member, _ := app.coreAPI.Member(bot.MemberToken); !body.Enabled || !member.Active {
		t.Error("re-enabling did not reactivate the member")
	}
}

func TestDeletingABotRemovesItsMember(t *testing.T) {
	app := newTestApp(t)
	bot := app.createBotOn(app.claudeAccount().Token, nil)

	expectStatus(t, app.do(request{method: "DELETE", path: "/api/genagents/v1/bots/" + bot.Token}), http.StatusNoContent)

	if _, found := app.coreAPI.Member(bot.MemberToken); found {
		t.Error("the member still exists")
	}
	var ciphertext []byte
	if err := app.db.QueryRow(context.Background(), `SELECT member_secret_ciphertext FROM bots WHERE token = $1`, bot.Token).Scan(&ciphertext); err != nil {
		t.Fatalf("read stored bot: %v", err)
	}
	if ciphertext != nil {
		t.Error("a removed bot kept its secret")
	}
	expectError(t, app.do(request{method: "GET", path: "/api/genagents/v1/bots/" + bot.Token}), http.StatusNotFound, "bot_not_found")
	if resp := app.createBotRequest(devBotBody(app.createAPIKeyAccount("Other", "openai", "sk-other-0001").Token)); resp.status == http.StatusConflict {
		t.Error("the name of a removed bot stayed taken")
	}
}

func TestRotatingTheSecretKeepsTheBotSigningIn(t *testing.T) {
	app := newTestApp(t)
	bot := app.createBotOn(app.claudeAccount().Token, nil)
	before, _ := app.coreAPI.Member(bot.MemberToken)

	expectStatus(t, app.do(request{method: "POST", path: "/api/genagents/v1/bots/" + bot.Token + "/secret"}), http.StatusNoContent)

	after, _ := app.coreAPI.Member(bot.MemberToken)
	if after.Secret == before.Secret {
		t.Error("core kept the old secret")
	}
	expectError(t, app.do(request{method: "POST", path: "/api/genagents/v1/bots/missing/secret"}), http.StatusNotFound, "bot_not_found")
}

func TestRemovingAnAccountInUseNamesTheBots(t *testing.T) {
	app := newTestApp(t)
	account := app.claudeAccount()
	app.createBotOn(account.Token, nil)

	resp := app.do(request{method: "DELETE", path: "/api/genagents/v1/accounts/" + account.Token})

	body := expectError(t, resp, http.StatusConflict, "account_in_use")
	if !strings.Contains(body.Error.Fields["bots"], "Dev Bot") {
		t.Errorf("fields = %v", body.Error.Fields)
	}
}

func TestMembersListBotsByKindWithoutCredentials(t *testing.T) {
	app := newTestApp(t)
	account := app.claudeAccount()
	app.createBotOn(account.Token, nil)
	app.createBotOn(account.Token, func(body map[string]any) { body["name"], body["kind"] = "Review Bot", "reviewer" })
	member := app.tokenWith(access.PermissionBotRead)

	resp := app.do(request{method: "GET", path: "/api/genagents/v1/bots?kind=reviewer", accessToken: member})

	expectStatus(t, resp, http.StatusOK)
	var body struct {
		Bots []botBody `json:"bots"`
	}
	resp.decode(t, &body)
	if len(body.Bots) != 1 || body.Bots[0].Name != "Review Bot" || body.Bots[0].MemberToken == "" || body.Bots[0].Provider != "anthropic" {
		t.Errorf("bots = %+v", body.Bots)
	}
	expectValidationError(t, app.do(request{method: "GET", path: "/api/genagents/v1/bots?kind=boss"}), "kind")
}

func TestBotManagementNeedsThePermission(t *testing.T) {
	app := newTestApp(t)
	member := app.tokenWith(access.PermissionBotRead)

	for _, call := range []request{
		{method: "POST", path: "/api/genagents/v1/bots", body: map[string]any{}},
		{method: "PATCH", path: "/api/genagents/v1/bots/x", body: map[string]any{}},
		{method: "DELETE", path: "/api/genagents/v1/bots/x"},
		{method: "POST", path: "/api/genagents/v1/bots/x/secret"},
	} {
		call.accessToken = member
		expectError(t, app.do(call), http.StatusForbidden, "forbidden")
	}
	expectError(t, app.do(request{method: "GET", path: "/api/genagents/v1/bots", accessToken: app.tokenWith()}), http.StatusForbidden, "forbidden")
}
