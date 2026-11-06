package execution_test

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/execution"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/genagentstest"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/sandbox"
)

func TestDevRunPushesFromTheHostAndOpensThePullRequest(t *testing.T) {
	env := newEnvironment(t, commitScript(prJSON), nil)

	env.execute(genagentstest.TaskRun("run-1"))

	result := env.lastEvent("run-1")
	if result.Type != "result" || result.Payload["pullRequestUrl"] != "https://github.com/acme/api/pull/12" || result.Payload["sessionId"] != "cli-session-1" {
		t.Fatalf("last event = %+v", result)
	}
	if got := strings.Join(env.eventTypes("run-1"), ","); got != "assistant_message,tool_use,result" {
		t.Errorf("events = %s", got)
	}
	if len(env.github.PullRequests) != 1 {
		t.Fatalf("pull requests = %v", env.github.PullRequests)
	}
	pullRequest := env.github.PullRequests[0]
	if pullRequest["title"] != "GEN-1 Speed up the API" || pullRequest["head"] != "bot/gen-1-abc123" || pullRequest["base"] != "main" || pullRequest["body"] != "Faster orders" {
		t.Errorf("pull request = %v", pullRequest)
	}
	if !contains(env.remote.Branches(t), "bot/gen-1-abc123") {
		t.Errorf("the branch was not pushed: %v", env.remote.Branches(t))
	}
	if author := env.remote.LastCommitAuthor(t, "bot/gen-1-abc123"); !strings.Contains(author, "Dev Bot") || !strings.Contains(author, "bots.genwork.invalid") {
		t.Errorf("commit author = %s", author)
	}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func TestSandboxGetsOnlyTheRunsOwnInputs(t *testing.T) {
	env := newEnvironment(t, commitScript(prJSON), nil)

	env.execute(genagentstest.TaskRun("run-1"))

	spec := env.launcher.Specs[0]
	if spec.Image != "registry.example/genwork-sandbox:2.0.0" || spec.RunToken != "run-1" || spec.Profile != nil || spec.Env["ANTHROPIC_API_KEY"] != "sk-ant-run-secret-0001" {
		t.Errorf("spec = %+v", spec)
	}
	encoded, _ := json.Marshal(spec)
	for _, forbidden := range []string{"ghp-host-only-token", genagentstest.Secret, "runner-token"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Errorf("the sandbox spec holds %s: %s", forbidden, encoded)
		}
	}
	if !strings.Contains(strings.Join(spec.Command, " "), "--resume") && !strings.Contains(strings.Join(spec.Command, " "), "claude -p") {
		t.Errorf("command = %v", spec.Command)
	}
}

func TestSubscriptionRunMountsOnlyItsAccountProfile(t *testing.T) {
	env := newEnvironment(t, commitScript(prJSON), nil)
	run := genagentstest.TaskRun("run-1")
	run["account"] = map[string]any{"token": "acct-team", "provider": "anthropic", "mode": "subscription", "apiKey": ""}

	env.execute(run)

	spec := env.launcher.Specs[0]
	wantDir := filepath.Join(env.root, "profiles", "acct-team")
	if spec.Profile == nil || spec.Profile.Dir != wantDir || spec.Profile.Mount != "/home/sandbox/.claude" || spec.Env["CLAUDE_CONFIG_DIR"] != "/home/sandbox/.claude" {
		t.Errorf("profile = %+v, env = %v", spec.Profile, spec.Env)
	}
	if info, err := os.Stat(wantDir); err != nil || !info.IsDir() {
		t.Errorf("the profile directory was not created: %v", err)
	}
}

func TestWorkspaceAndSecretFilesAreRemovedWhenTheRunEnds(t *testing.T) {
	var workspaceDir string
	script := func(ctx context.Context, turn int, spec sandbox.Spec, output io.Writer) (sandbox.Result, error) {
		workspaceDir = spec.Workspace
		if _, err := os.Stat(filepath.Join(spec.Workspace, ".genwork", "mcp.json")); err != nil {
			t.Errorf("the MCP config is missing during the run: %v", err)
		}
		return commitScript(prJSON)(ctx, turn, spec, output)
	}
	env := newEnvironment(t, script, nil)

	env.execute(genagentstest.TaskRun("run-1"))

	if workspaceDir == "" {
		t.Fatal("the script never ran")
	}
	if _, err := os.Stat(workspaceDir); !os.IsNotExist(err) {
		t.Errorf("the workspace survived the run: %v", err)
	}
}

func TestRunWithNoCommitsAndAQuestionWaitsForAReplyThenResumes(t *testing.T) {
	var env *environment
	script := func(ctx context.Context, turn int, spec sandbox.Spec, output io.Writer) (sandbox.Result, error) {
		if turn == 1 {
			_, err := io.WriteString(output, resultLine("Should I change the public API or add a new endpoint?"))
			go func() {
				time.Sleep(150 * time.Millisecond)
				env.genagents.QueueMessage("run-1", 1, "add a new endpoint")
			}()
			return sandbox.Result{}, err
		}
		return commitScript(prJSON)(ctx, turn, spec, output)
	}
	env = newEnvironment(t, script, nil)

	env.execute(genagentstest.TaskRun("run-1"))

	var states []string
	for _, event := range env.genagents.EventsOf("run-1") {
		if event.Type == "status" {
			states = append(states, event.Payload["state"].(string))
		}
	}
	if strings.Join(states, ",") != "waiting_for_input,running" {
		t.Errorf("states = %v", states)
	}
	if env.launcher.Turns() != 2 {
		t.Fatalf("turns = %d", env.launcher.Turns())
	}
	second := strings.Join(env.launcher.Specs[1].Command, " ")
	if !strings.Contains(second, "--resume cli-session-1") || !strings.Contains(second, "add a new endpoint") {
		t.Errorf("the second turn = %s", second)
	}
	if env.genagents.Acked["run-1"] != 1 || env.lastEvent("run-1").Type != "result" || len(env.github.PullRequests) != 1 {
		t.Errorf("acked = %d, last = %+v", env.genagents.Acked["run-1"], env.lastEvent("run-1"))
	}
}

func TestMessageArrivingDuringATurnIsDeliveredAtTheNextTurnBoundary(t *testing.T) {
	var env *environment
	script := func(ctx context.Context, turn int, spec sandbox.Spec, output io.Writer) (sandbox.Result, error) {
		if turn == 1 {
			env.genagents.QueueMessage("run-1", 1, "use the existing TaskRepository")
		}
		return commitScript(prJSON)(ctx, turn, spec, output)
	}
	env = newEnvironment(t, script, nil)

	env.execute(genagentstest.TaskRun("run-1"))

	if env.launcher.Turns() != 2 {
		t.Fatalf("turns = %d", env.launcher.Turns())
	}
	if !strings.Contains(strings.Join(env.launcher.Specs[1].Command, " "), "use the existing TaskRepository") {
		t.Errorf("the message was not delivered: %v", env.launcher.Specs[1].Command)
	}
}

func TestNoCommitsWithADraftReportsNoChangesAndPushesNothing(t *testing.T) {
	env := newEnvironment(t, func(_ context.Context, _ int, _ sandbox.Spec, output io.Writer) (sandbox.Result, error) {
		_, err := io.WriteString(output, resultLine(`Nothing to change. {"pullRequestTitle": "GEN-1 nothing", "pullRequestBody": "n/a"}`))
		return sandbox.Result{}, err
	}, nil)

	env.execute(genagentstest.TaskRun("run-1"))

	result := env.lastEvent("run-1")
	if result.Type != "result" || result.Payload["noChanges"] != true || len(env.github.PullRequests) != 0 || contains(env.remote.Branches(t), "bot/gen-1-abc123") {
		t.Errorf("result = %+v, pull requests = %v, branches = %v", result, env.github.PullRequests, env.remote.Branches(t))
	}
}

func TestPullRequestTitleAlwaysCitesTheTaskKey(t *testing.T) {
	env := newEnvironment(t, commitScript(`Done. {"pullRequestTitle": "Speed up orders", "pullRequestBody": "x"}`), nil)

	env.execute(genagentstest.TaskRun("run-1"))

	if title := env.github.PullRequests[0]["title"]; title != "GEN-1 Speed up orders" {
		t.Errorf("title = %q", title)
	}
}

func TestPushOrPullRequestFailureFailsTheRunWithGithubPostFailed(t *testing.T) {
	env := newEnvironment(t, commitScript(prJSON), nil)
	env.github.FailPR = true

	env.execute(genagentstest.TaskRun("run-1"))

	last := env.lastEvent("run-1")
	if last.Type != "error" || last.Payload["reason"] != "github_post_failed" {
		t.Errorf("last event = %+v", last)
	}
}

func TestTimeoutStopsTheSandboxAndFailsTheRun(t *testing.T) {
	env := newEnvironment(t, func(ctx context.Context, _ int, _ sandbox.Spec, _ io.Writer) (sandbox.Result, error) {
		<-ctx.Done()
		return sandbox.Result{ExitCode: 137}, nil
	}, func(settings *execution.Settings) { settings.TimeoutUnit = 100 * time.Millisecond })
	run := genagentstest.TaskRun("run-1")
	run["timeoutMinutes"] = 1

	env.execute(run)

	last := env.lastEvent("run-1")
	if last.Type != "error" || last.Payload["reason"] != "timeout" || len(env.launcher.Stopped) != 1 || env.launcher.Stopped[0] != "run-1" {
		t.Errorf("last = %+v, stopped = %v", last, env.launcher.Stopped)
	}
}

func TestOutOfMemoryFailsTheRunWithTheSandboxLimit(t *testing.T) {
	env := newEnvironment(t, func(context.Context, int, sandbox.Spec, io.Writer) (sandbox.Result, error) {
		return sandbox.Result{ExitCode: 137, OOMKilled: true}, nil
	}, nil)

	env.execute(genagentstest.TaskRun("run-1"))

	last := env.lastEvent("run-1")
	if last.Payload["reason"] != "sandbox_limit" || last.Payload["limit"] != "memory" {
		t.Errorf("last = %+v", last)
	}
}

func TestProviderAuthFailureIsReportedAsAnAccountError(t *testing.T) {
	env := newEnvironment(t, func(_ context.Context, _ int, _ sandbox.Spec, output io.Writer) (sandbox.Result, error) {
		_, err := io.WriteString(output, `{"type":"system","subtype":"init","session_id":"s"}`+"\n"+`{"type":"result","subtype":"error_during_execution","is_error":true,"result":"Invalid API key sk-ant-run-secret-0001","session_id":"s"}`+"\n")
		return sandbox.Result{ExitCode: 1}, err
	}, nil)

	env.execute(genagentstest.TaskRun("run-1"))

	last := env.lastEvent("run-1")
	if last.Payload["reason"] != "account_error" || last.Payload["authFailure"] != true {
		t.Errorf("last = %+v", last)
	}
	if strings.Contains(last.Payload["message"].(string), "sk-ant-run-secret-0001") {
		t.Errorf("the key reached the message: %v", last.Payload["message"])
	}
}

func TestUnparseableOutputFailsWithAdapterErrorAndTheRawTail(t *testing.T) {
	env := newEnvironment(t, func(_ context.Context, _ int, _ sandbox.Spec, output io.Writer) (sandbox.Result, error) {
		_, err := io.WriteString(output, "Segmentation fault")
		return sandbox.Result{ExitCode: 139}, err
	}, nil)

	env.execute(genagentstest.TaskRun("run-1"))

	last := env.lastEvent("run-1")
	if last.Payload["reason"] != "adapter_error" || !strings.Contains(last.Payload["message"].(string), "Segmentation fault") {
		t.Errorf("last = %+v", last)
	}
}

func TestSandboxLaunchErrorsFailTheRun(t *testing.T) {
	env := newEnvironment(t, func(context.Context, int, sandbox.Spec, io.Writer) (sandbox.Result, error) {
		return sandbox.Result{}, sandbox.ErrNoEngine
	}, nil)

	env.execute(genagentstest.TaskRun("run-1"))

	if last := env.lastEvent("run-1"); last.Payload["reason"] != "adapter_error" {
		t.Errorf("last = %+v", last)
	}
}

func TestACancelledRunStopsAndPostsNothingMore(t *testing.T) {
	var env *environment
	script := func(ctx context.Context, _ int, _ sandbox.Spec, output io.Writer) (sandbox.Result, error) {
		_, _ = io.WriteString(output, `{"type":"assistant","message":{"content":[{"type":"text","text":"working"}]}}`+"\n")
		env.genagents.SetRunState("run-1", "cancelled")
		<-ctx.Done()
		return sandbox.Result{ExitCode: 143}, nil
	}
	env = newEnvironment(t, script, nil)

	env.execute(genagentstest.TaskRun("run-1"))

	for _, event := range env.genagents.EventsOf("run-1") {
		if event.Type == "error" || event.Type == "result" {
			t.Errorf("a cancelled run posted %s", event.Type)
		}
	}
}

func TestEventsAreRetriedWithTheSameSequence(t *testing.T) {
	env := newEnvironment(t, commitScript(prJSON), nil)
	env.genagents.FailNextEvents(2)

	env.execute(genagentstest.TaskRun("run-1"))

	sequences := map[int64]int{}
	for _, event := range env.genagents.EventsOf("run-1") {
		sequences[event.Sequence]++
	}
	for sequence, count := range sequences {
		if count != 1 {
			t.Errorf("sequence %d was stored %d times", sequence, count)
		}
	}
	if env.lastEvent("run-1").Type != "result" {
		t.Errorf("events = %v", env.eventTypes("run-1"))
	}
}

func TestSecretsNeverReachTheConversation(t *testing.T) {
	env := newEnvironment(t, func(_ context.Context, _ int, _ sandbox.Spec, output io.Writer) (sandbox.Result, error) {
		_, err := io.WriteString(output, `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"echo sk-ant-run-secret-0001 ghp_abcdefghijklmnop"}}]}}`+"\n"+
			`{"type":"result","subtype":"success","result":"ok","session_id":"s"}`+"\n")
		return sandbox.Result{}, err
	}, nil)
	run := genagentstest.ChatRun("run-1")

	env.execute(run)

	encoded, _ := json.Marshal(env.genagents.EventsOf("run-1"))
	for _, secret := range []string{"sk-ant-run-secret-0001", "ghp_abcdefghijklmnop", "ghp-host-only-token"} {
		if strings.Contains(string(encoded), secret) {
			t.Errorf("a secret reached the conversation: %s", encoded)
		}
	}
}

func TestChatRunHasNoRepositoryAndEndsWithItsAnswer(t *testing.T) {
	env := newEnvironment(t, func(_ context.Context, _ int, spec sandbox.Spec, output io.Writer) (sandbox.Result, error) {
		if _, err := os.Stat(filepath.Join(spec.Workspace, ".git")); err == nil {
			t.Error("a chat run got a repository")
		}
		_, err := io.WriteString(output, resultLine("You have 3 tasks."))
		return sandbox.Result{}, err
	}, nil)

	env.execute(genagentstest.ChatRun("run-1"))

	result := env.lastEvent("run-1")
	if result.Type != "result" || result.Payload["summary"] != "You have 3 tasks." || result.Payload["sessionId"] != "cli-session-1" {
		t.Errorf("result = %+v", result)
	}
}

func TestResumedRunsContinueTheirCLISession(t *testing.T) {
	env := newEnvironment(t, func(_ context.Context, _ int, _ sandbox.Spec, output io.Writer) (sandbox.Result, error) {
		_, err := io.WriteString(output, resultLine("ok"))
		return sandbox.Result{}, err
	}, nil)
	run := genagentstest.ChatRun("run-1")
	run["resumeSession"] = "cli-previous"

	env.execute(run)

	if !strings.Contains(strings.Join(env.launcher.Specs[0].Command, " "), "--resume cli-previous") {
		t.Errorf("command = %v", env.launcher.Specs[0].Command)
	}
}

func TestGenworkTokenIsRenewedWhileTheRunLasts(t *testing.T) {
	renewed := false
	env := newEnvironment(t, func(ctx context.Context, _ int, spec sandbox.Spec, output io.Writer) (sandbox.Result, error) {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) && !renewed {
			content, _ := os.ReadFile(filepath.Join(spec.Workspace, ".genwork", "mcp.json"))
			renewed = strings.Contains(string(content), "genwork-access-token-c")
			time.Sleep(20 * time.Millisecond)
		}
		_, err := io.WriteString(output, resultLine("ok"))
		return sandbox.Result{}, err
	}, func(settings *execution.Settings) { settings.TokenRefresh = 50 * time.Millisecond })

	env.execute(genagentstest.ChatRun("run-1"))

	if !renewed || env.genagents.TokenRequests() < 2 {
		t.Errorf("renewed = %v, token requests = %d", renewed, env.genagents.TokenRequests())
	}
}

func TestReviewPostsACommentReviewPerPullRequest(t *testing.T) {
	reviewJSON := `{"pullRequests": [{"repository": "acme/api", "number": 12, "summary": "Mostly fine", "findings": [` +
		`{"path": "orders.go", "line": 3, "severity": "warning", "message": "missing nil check"},` +
		`{"path": "orders.go", "line": 9, "severity": "error", "message": "race condition"}]}]}`
	env := newEnvironment(t, func(_ context.Context, _ int, spec sandbox.Spec, output io.Writer) (sandbox.Result, error) {
		if _, err := os.Stat(filepath.Join(spec.Workspace, "repos", "acme__api", "feature.txt")); err != nil {
			t.Errorf("the pull request head is not checked out: %v", err)
		}
		_, err := io.WriteString(output, resultLine(reviewJSON))
		return sandbox.Result{}, err
	}, nil)
	env.remote.AddPullRequestHead(t, 12, "feature.txt", "reviewed")

	env.execute(genagentstest.ReviewRun("run-1"))

	if len(env.github.Reviews) != 1 {
		t.Fatalf("reviews = %v", env.github.Reviews)
	}
	review := env.github.Reviews[0]
	comments := review["comments"].([]execution.ReviewComment)
	if review["repository"] != "acme/api" || review["number"] != 12 || review["summary"] != "Mostly fine" || len(comments) != 2 ||
		!strings.Contains(comments[1].Body, "ERROR") || comments[0].Line != 3 {
		t.Errorf("review = %+v", review)
	}
	result := env.lastEvent("run-1")
	if result.Type != "result" || result.Payload["findings"].(float64) != 2 || result.Payload["summary"] != "2 findings" {
		t.Errorf("result = %+v", result)
	}
	if prompt := strings.Join(env.launcher.Specs[0].Command, " "); !strings.Contains(prompt, "repos/acme__api") || !strings.Contains(prompt, "pr-<number>") {
		t.Errorf("the prompt does not say where the pull requests are: %s", prompt)
	}
}

func TestReviewJSONIsValidatedBeforeAnythingIsPosted(t *testing.T) {
	for name, finalText := range map[string]string{
		"no json":      "Looks good to me",
		"unknown pr":   `{"pullRequests": [{"repository": "acme/api", "number": 99, "summary": "x", "findings": []}]}`,
		"bad severity": `{"pullRequests": [{"repository": "acme/api", "number": 12, "summary": "x", "findings": [{"path": "a", "line": 1, "severity": "blocker", "message": "m"}]}]}`,
		"missing line": `{"pullRequests": [{"repository": "acme/api", "number": 12, "summary": "x", "findings": [{"path": "a", "severity": "info", "message": "m"}]}]}`,
		"empty list":   `{"pullRequests": []}`,
	} {
		t.Run(name, func(t *testing.T) {
			env := newEnvironment(t, func(_ context.Context, _ int, _ sandbox.Spec, output io.Writer) (sandbox.Result, error) {
				_, err := io.WriteString(output, resultLine(finalText))
				return sandbox.Result{}, err
			}, nil)
			env.remote.AddPullRequestHead(t, 12, "feature.txt", "x")

			env.execute(genagentstest.ReviewRun("run-1"))

			if last := env.lastEvent("run-1"); last.Type != "error" || last.Payload["reason"] != "adapter_error" || len(env.github.Reviews) != 0 {
				t.Errorf("last = %+v, reviews = %v", last, env.github.Reviews)
			}
		})
	}
}

func TestGithubRefusingTheReviewPostsItsFullTextToTheConversation(t *testing.T) {
	reviewJSON := `{"pullRequests": [{"repository": "acme/api", "number": 12, "summary": "Two issues", "findings": [{"path": "a.go", "line": 2, "severity": "info", "message": "rename this"}]}]}`
	env := newEnvironment(t, func(_ context.Context, _ int, _ sandbox.Spec, output io.Writer) (sandbox.Result, error) {
		_, err := io.WriteString(output, resultLine(reviewJSON))
		return sandbox.Result{}, err
	}, nil)
	env.remote.AddPullRequestHead(t, 12, "feature.txt", "x")
	env.github.FailReview = true

	env.execute(genagentstest.ReviewRun("run-1"))

	events := env.genagents.EventsOf("run-1")
	full := events[len(events)-2]
	last := events[len(events)-1]
	if full.Type != "assistant_message" || !strings.Contains(full.Payload["text"].(string), "rename this") || !strings.Contains(full.Payload["text"].(string), "acme/api#12") {
		t.Errorf("full review event = %+v", full)
	}
	if last.Type != "error" || last.Payload["reason"] != "github_post_failed" {
		t.Errorf("last = %+v", last)
	}
}

func TestCloneFailureFailsTheRunBeforeAnySandboxStarts(t *testing.T) {
	env := newEnvironment(t, commitScript(prJSON), nil)
	run := genagentstest.TaskRun("run-1")
	run["repository"] = "acme/does-not-exist"

	env.execute(run)

	last := env.lastEvent("run-1")
	if last.Payload["reason"] != "adapter_error" || env.launcher.Turns() != 0 {
		t.Errorf("last = %+v, turns = %d", last, env.launcher.Turns())
	}
	if strings.Contains(last.Payload["message"].(string), "ghp-host-only-token") {
		t.Error("the GitHub token reached the conversation")
	}
}

func TestUnknownProviderFailsTheRun(t *testing.T) {
	env := newEnvironment(t, commitScript(prJSON), nil)
	run := genagentstest.TaskRun("run-1")
	run["account"] = map[string]any{"token": "a", "provider": "mystery", "mode": "api_key", "apiKey": "k"}

	env.execute(run)

	if last := env.lastEvent("run-1"); last.Payload["reason"] != "adapter_error" {
		t.Errorf("last = %+v", last)
	}
}
