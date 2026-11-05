package app_test

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runs"
)

func TestClaimReturnsTheRunSpecWithSecretsOnce(t *testing.T) {
	app := newTestApp(t)
	runner := app.registerRunner("build-box", heartbeatOptions{tools: []string{"claude"}, github: true})
	skill := app.createGoSkill()
	server := app.createSentry()
	instructions := "Use the repository conventions"
	bot := app.createBotOn(app.claudeAccount().Token, func(body map[string]any) {
		body["skillTokens"], body["mcpServerTokens"], body["instructions"] = []string{skill.Token}, []string{server.Token}, instructions
	})
	run := app.queueRun(bot.Token, runs.RunKindTask, "task-1")

	claimed, resp := app.claim(runner, 1)

	if claimed == nil {
		t.Fatalf("no run claimed: %d %s", resp.status, resp.body)
	}
	if claimed.RunToken != run.Token || claimed.Kind != "task" || claimed.Prompt != "Do the task" || claimed.Repository != "acme/api" ||
		claimed.Branch != "bot/gen-1-abc" || claimed.Model != "claude-sonnet-5-5" || claimed.Instructions != instructions ||
		claimed.GenworkMCPURL != "http://genwork.test/mcp" || claimed.Account.APIKey != "sk-ant-team-0001" || claimed.Account.Mode != "api_key" {
		t.Errorf("claimed = %+v", claimed)
	}
	if len(claimed.Skills) != 1 || claimed.Skills[0].Name != "go-conventions" || len(claimed.Skills[0].Files) != 1 {
		t.Errorf("skills = %+v", claimed.Skills)
	}
	if len(claimed.MCPServers) != 1 || len(claimed.MCPServers[0].Headers) != 2 || claimed.MCPServers[0].Headers[0].Value != "Bearer top-secret" {
		t.Errorf("mcp servers = %+v", claimed.MCPServers)
	}
	var spec string
	if err := app.db.QueryRow(context.Background(), `SELECT spec::text FROM runs WHERE token = $1`, run.Token).Scan(&spec); err != nil {
		t.Fatalf("read stored spec: %v", err)
	}
	for _, secret := range []string{"sk-ant-team-0001", "top-secret"} {
		if strings.Contains(spec, secret) {
			t.Errorf("the stored spec holds a secret: %s", spec)
		}
	}
	if !strings.Contains(spec, skill.Token) || app.runState(run.Token) != "running" {
		t.Errorf("stored spec = %s, state = %s", spec, app.runState(run.Token))
	}
	if again, _ := app.claim(runner, 1); again != nil {
		t.Error("the same run was claimed twice")
	}
}

func TestClaimWithNothingQueuedAnswersNoContent(t *testing.T) {
	app := newTestApp(t)
	runner := app.registerRunner("build-box", heartbeatOptions{tools: []string{"claude"}, github: true})

	_, resp := app.claim(runner, 1)

	expectNoContent(t, resp)
}

func TestTwoRunnersNeverClaimTheSameRun(t *testing.T) {
	app := newTestApp(t)
	first := app.registerRunner("first", heartbeatOptions{tools: []string{"claude"}, github: true})
	second := app.registerRunner("second", heartbeatOptions{tools: []string{"claude"}, github: true})
	bot := app.createBotOn(app.claudeAccount().Token, nil)
	app.queueRun(bot.Token, runs.RunKindTask, "task-1")

	var wait sync.WaitGroup
	results := make([]*claimBody, 2)
	for index, runner := range []testRunner{first, second} {
		wait.Add(1)
		go func() {
			defer wait.Done()
			results[index], _ = app.claim(runner, 1)
		}()
	}
	wait.Wait()

	winners := 0
	for _, result := range results {
		if result != nil {
			winners++
		}
	}
	if winners != 1 {
		t.Errorf("%d runners claimed the run", winners)
	}
}

func TestClaimsFollowEligibility(t *testing.T) {
	app := newTestApp(t)
	withoutGithub := app.registerRunner("no-github", heartbeatOptions{tools: []string{"claude"}})
	withoutCLI := app.registerRunner("no-cli", heartbeatOptions{tools: []string{"codex"}, github: true})
	capable := app.registerRunner("capable", heartbeatOptions{tools: []string{"claude"}, github: true})
	bot := app.createBotOn(app.claudeAccount().Token, nil)
	app.queueRun(bot.Token, runs.RunKindTask, "task-1")

	if claimed, _ := app.claim(withoutGithub, 1); claimed != nil {
		t.Error("a runner without GitHub credentials claimed a task run")
	}
	if claimed, _ := app.claim(withoutCLI, 1); claimed != nil {
		t.Error("a runner without the provider CLI claimed the run")
	}
	if claimed, _ := app.claim(capable, 1); claimed == nil {
		t.Error("the capable runner got nothing")
	}
}

func TestRunnerWithoutGithubStillTakesChatRuns(t *testing.T) {
	app := newTestApp(t)
	runner := app.registerRunner("no-github", heartbeatOptions{tools: []string{"claude"}})
	bot := app.createBotOn(app.claudeAccount().Token, func(body map[string]any) { body["kind"] = "assistant" })
	app.queueRun(bot.Token, runs.RunKindChat, "task-chat")

	if claimed, _ := app.claim(runner, 1); claimed == nil {
		t.Error("a runner without GitHub credentials did not get the chat run")
	}
}

func TestSubscriptionRunsGoOnlyToTheRunnerHoldingTheAccount(t *testing.T) {
	app := newTestApp(t)
	holder := app.registerRunner("holder", heartbeatOptions{tools: []string{"claude"}, github: true})
	other := app.registerRunner("other", heartbeatOptions{tools: []string{"claude"}, github: true})
	var account accountBody
	app.createAccount(map[string]any{"name": "Claude Pro", "provider": "anthropic", "mode": "subscription", "runnerToken": holder.Token}).decode(t, &account)
	bot := app.createBotOn(account.Token, nil)
	app.queueRun(bot.Token, runs.RunKindTask, "task-1")

	if claimed, _ := app.claim(holder, 1); claimed != nil {
		t.Error("a run was claimed before the account signed in")
	}
	expectNoContent(t, app.runnerDo(holder.Secret, "POST", "/api/genagents/v1/runner/accounts/"+account.Token+"/status", map[string]any{"signedIn": true}))
	if claimed, _ := app.claim(other, 1); claimed != nil {
		t.Error("a runner without the account claimed its run")
	}
	claimed, _ := app.claim(holder, 1)
	if claimed == nil || claimed.Account.Mode != "subscription" || claimed.Account.APIKey != "" {
		t.Errorf("claimed = %+v", claimed)
	}
}

func TestClaimsRespectFreeSlotsAndConcurrency(t *testing.T) {
	app := newTestApp(t)
	runner := app.registerRunner("build-box", heartbeatOptions{tools: []string{"claude"}, github: true, maxConcurrency: 1})
	bot := app.createBotOn(app.claudeAccount().Token, nil)
	app.queueRun(bot.Token, runs.RunKindTask, "task-1")
	app.queueRun(bot.Token, runs.RunKindTask, "task-2")

	if claimed, _ := app.claim(runner, 0); claimed != nil {
		t.Error("a claim without free slots succeeded")
	}
	if claimed, _ := app.claim(runner, 1); claimed == nil {
		t.Fatal("the first claim failed")
	}
	if claimed, _ := app.claim(runner, 1); claimed != nil {
		t.Error("a runner went over its maximum concurrency")
	}
}

func TestClaimsGoToTheOldestQueuedRunFirst(t *testing.T) {
	app := newTestApp(t)
	runner := app.registerRunner("build-box", heartbeatOptions{tools: []string{"claude"}, github: true})
	bot := app.createBotOn(app.claudeAccount().Token, nil)
	older := app.queueRun(bot.Token, runs.RunKindTask, "task-1")
	app.clock.Advance(time.Minute)
	app.queueRun(bot.Token, runs.RunKindTask, "task-2")

	claimed, _ := app.claim(runner, 1)

	if claimed == nil || claimed.RunToken != older.Token {
		t.Errorf("claimed = %+v, want %s", claimed, older.Token)
	}
}

func TestDirectoryModeRunnerOnlyGetsWorkNoSandboxedRunnerCanTake(t *testing.T) {
	app := newTestApp(t)
	directory := app.registerRunner("dev-box", heartbeatOptions{tools: []string{"claude"}, github: true, directoryMode: true})
	sandboxed := app.registerRunner("sandboxed", heartbeatOptions{tools: []string{"claude"}, github: true})
	bot := app.createBotOn(app.claudeAccount().Token, nil)
	app.queueRun(bot.Token, runs.RunKindTask, "task-1")

	if claimed, _ := app.claim(directory, 1); claimed != nil {
		t.Fatal("a directory-mode runner took work a sandboxed runner can take")
	}
	if claimed, _ := app.claim(sandboxed, 1); claimed == nil {
		t.Fatal("the sandboxed runner got nothing")
	}
	app.queueRun(bot.Token, runs.RunKindTask, "task-2")
	app.clock.Advance(2 * time.Minute)
	if claimed, _ := app.claim(directory, 1); claimed == nil {
		t.Error("with the sandboxed runner offline, the directory-mode runner got nothing")
	}
}

func TestAccountInErrorFailsQueuedRunsOnClaim(t *testing.T) {
	app := newTestApp(t)
	runner := app.registerRunner("build-box", heartbeatOptions{tools: []string{"claude"}, github: true})
	account := app.claudeAccount()
	bot := app.createBotOn(account.Token, nil)
	run := app.queueRun(bot.Token, runs.RunKindTask, "task-1")
	if _, err := app.db.Exec(context.Background(), `UPDATE ai_accounts SET status = 'error', last_error = 'invalid x-api-key' WHERE token = $1`, account.Token); err != nil {
		t.Fatalf("break account: %v", err)
	}

	app.claim(runner, 1)

	if app.runState(run.Token) != "failed" || app.runFailureReason(run.Token) != "account_error" {
		t.Errorf("state = %s (%s)", app.runState(run.Token), app.runFailureReason(run.Token))
	}
}

func TestDisabledBotRunsAreNotClaimed(t *testing.T) {
	app := newTestApp(t)
	runner := app.registerRunner("build-box", heartbeatOptions{tools: []string{"claude"}, github: true})
	bot := app.createBotOn(app.claudeAccount().Token, nil)
	run := app.queueRun(bot.Token, runs.RunKindTask, "task-1")
	expectStatus(t, app.do(request{method: "PATCH", path: "/api/genagents/v1/bots/" + bot.Token, body: map[string]any{"enabled": false}}), http.StatusOK)

	claimed, _ := app.claim(runner, 1)

	if claimed != nil || app.runState(run.Token) != "cancelled" {
		t.Errorf("claimed = %+v, state = %s", claimed, app.runState(run.Token))
	}
}
