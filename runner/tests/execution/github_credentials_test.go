package execution_test

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/genagentstest"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/sandbox"
)

const platformGithubToken = "ghs_platformIssuedToken1234567890"

func TestADevRunUsesTheGithubTokenThePlatformIssues(t *testing.T) {
	env := newEnvironmentWithRunnerToken(t, commitScript(prJSON), nil, "")
	env.genagents.SetGithubToken("run-1", platformGithubToken)

	env.execute(genagentstest.TaskRun("run-1"))

	if result := env.lastEvent("run-1"); result.Type != "result" {
		t.Fatalf("last event = %+v", result)
	}
	if len(env.github.Tokens) != 1 || env.github.Tokens[0] != platformGithubToken {
		t.Errorf("github tokens = %v", env.github.Tokens)
	}
	if !contains(env.remote.Branches(t), "bot/gen-1-abc123") {
		t.Errorf("the branch was not pushed: %v", env.remote.Branches(t))
	}
	for _, event := range env.genagents.EventsOf("run-1") {
		if encoded, _ := json.Marshal(event.Payload); strings.Contains(string(encoded), platformGithubToken) {
			t.Errorf("the platform token reached the conversation: %+v", event)
		}
	}
	for _, spec := range env.launcher.Specs {
		for key, value := range spec.Env {
			if strings.Contains(value, platformGithubToken) {
				t.Errorf("the platform token entered the sandbox through %s", key)
			}
		}
	}
}

func TestADevRunFallsBackToTheRunnersOwnToken(t *testing.T) {
	env := newEnvironment(t, commitScript(prJSON), nil)

	env.execute(genagentstest.TaskRun("run-1"))

	if result := env.lastEvent("run-1"); result.Type != "result" {
		t.Fatalf("last event = %+v", result)
	}
	if len(env.github.Tokens) != 0 {
		t.Errorf("the executor switched tokens without a platform token: %v", env.github.Tokens)
	}
}

func TestADevRunWithoutAnyGithubCredentialsFailsBeforeCloning(t *testing.T) {
	env := newEnvironmentWithRunnerToken(t, commitScript(prJSON), nil, "")

	env.execute(genagentstest.TaskRun("run-1"))

	failure := env.lastEvent("run-1")
	if failure.Type != "error" || failure.Payload["reason"] != "github_credentials_missing" {
		t.Fatalf("last event = %+v", failure)
	}
	if env.launcher.Turns() != 0 {
		t.Error("a sandbox started without GitHub credentials")
	}
}

func TestAChatRunNeedsNoGithubCredentials(t *testing.T) {
	env := newEnvironmentWithRunnerToken(t, func(_ context.Context, _ int, _ sandbox.Spec, output io.Writer) (sandbox.Result, error) {
		_, err := io.WriteString(output, resultLine("done"))
		return sandbox.Result{}, err
	}, nil, "")

	env.execute(genagentstest.ChatRun("run-1"))

	if result := env.lastEvent("run-1"); result.Type != "result" {
		t.Fatalf("last event = %+v", result)
	}
}
