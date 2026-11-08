package execution_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/api"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/execution"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/genagentstest"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/gitremotetest"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/sandbox"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/sandboxtest"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/workspace"
)

// fakeGithub records what the executor opens and posts on GitHub, and can refuse.
type fakeGithub struct {
	mutex        sync.Mutex
	PullRequests []map[string]string
	Reviews      []map[string]any
	FailReview   bool
	FailPR       bool
	// Tokens lists the run tokens the executor switched to, in order.
	Tokens []string
	// Commentable is the diff's commentable lines per file; nil answers nil, as when the diff could not be read.
	Commentable map[string]map[int]bool
	// RejectInline makes GitHub refuse any review with inline comments, as it does for a line outside the diff.
	RejectInline bool
}

func (github *fakeGithub) CommentableLines(context.Context, string, int) (map[string]map[int]bool, error) {
	return github.Commentable, nil
}

func (github *fakeGithub) WithToken(token string) execution.GithubPoster {
	github.mutex.Lock()
	defer github.mutex.Unlock()
	github.Tokens = append(github.Tokens, token)
	return github
}

func (github *fakeGithub) DefaultBranch(context.Context, string) (string, error) { return "main", nil }

func (github *fakeGithub) CreatePullRequest(_ context.Context, repository, head, base, title, description string) (execution.PullRequestCreated, error) {
	github.mutex.Lock()
	defer github.mutex.Unlock()
	if github.FailPR {
		return execution.PullRequestCreated{}, fmt.Errorf("github refused the pull request")
	}
	github.PullRequests = append(github.PullRequests, map[string]string{"repository": repository, "head": head, "base": base, "title": title, "body": description})
	return execution.PullRequestCreated{Number: 12, URL: "https://github.com/" + repository + "/pull/12"}, nil
}

func (github *fakeGithub) CreateReview(_ context.Context, repository string, number int, summary string, comments []execution.ReviewComment) (string, error) {
	github.mutex.Lock()
	defer github.mutex.Unlock()
	if github.FailReview {
		return "", fmt.Errorf("github refused the review")
	}
	if github.RejectInline && len(comments) > 0 {
		return "", fmt.Errorf("%w: github answered 422: Unprocessable Entity: Line could not be resolved", execution.ErrReviewContentRejected)
	}
	github.Reviews = append(github.Reviews, map[string]any{"repository": repository, "number": number, "summary": summary, "comments": comments})
	return fmt.Sprintf("https://github.com/%s/pull/%d#review", repository, number), nil
}

type environment struct {
	t         *testing.T
	genagents *genagentstest.FakeGenagents
	remote    *gitremotetest.Remote
	launcher  *sandboxtest.FakeLauncher
	github    *fakeGithub
	workspace *workspace.Manager
	executor  *execution.Executor
	root      string
}

func newEnvironment(t *testing.T, script sandboxtest.Script, mutate func(*execution.Settings)) *environment {
	t.Helper()
	return newEnvironmentWithRunnerToken(t, script, mutate, "ghp-host-only-token")
}

// newEnvironmentWithRunnerToken sets the runner's own GitHub token; empty means the runner has none.
func newEnvironmentWithRunnerToken(t *testing.T, script sandboxtest.Script, mutate func(*execution.Settings), runnerToken string) *environment {
	t.Helper()
	env := &environment{t: t, genagents: genagentstest.NewFakeGenagents(t), remote: gitremotetest.NewRemote(t), github: &fakeGithub{}, root: t.TempDir()}
	env.launcher = sandboxtest.NewFakeLauncher(script)
	env.workspace = workspace.NewManager(filepath.Join(env.root, "runs"), env.remote.BaseURL, runnerToken)
	settings := execution.Settings{
		SandboxImage: "registry.example/genwork-sandbox:2.0.0", Limits: sandbox.Limits{CPUs: 2, Memory: 1 << 30, PIDs: 100},
		ProfileDir:   func(account string) string { return filepath.Join(env.root, "profiles", account) },
		PollInterval: 20 * time.Millisecond, TokenRefresh: time.Hour, Sleep: func(time.Duration) {}, TimeoutUnit: time.Minute,
	}
	if mutate != nil {
		mutate(&settings)
	}
	client := api.NewClient(env.genagents.URL(), genagentstest.Secret, nil)
	env.executor = execution.NewExecutor(client, env.launcher, env.workspace, env.github, settings)
	return env
}

// execute claims a queued run through the real API client and executes it to the end.
func (env *environment) execute(run map[string]any) {
	env.t.Helper()
	env.genagents.QueueRun(run)
	client := api.NewClient(env.genagents.URL(), genagentstest.Secret, nil)
	claimed, err := client.Claim(context.Background(), 1)
	if err != nil || claimed == nil {
		env.t.Fatalf("claim: %v / %v", claimed, err)
	}
	done := make(chan struct{})
	go func() {
		env.executor.Execute(context.Background(), *claimed)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		env.t.Fatal("the run did not finish")
	}
}

func (env *environment) eventTypes(runToken string) []string {
	var types []string
	for _, event := range env.genagents.EventsOf(runToken) {
		types = append(types, event.Type)
	}
	return types
}

func (env *environment) lastEvent(runToken string) genagentstest.PostedEvent {
	events := env.genagents.EventsOf(runToken)
	if len(events) == 0 {
		env.t.Fatalf("no events for %s", runToken)
	}
	return events[len(events)-1]
}

func runGit(directory string, arguments ...string) error {
	command := exec.Command("git", arguments...)
	command.Dir = directory
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("git %v: %w: %s", arguments, err, output)
	}
	return nil
}

const claudeSuccess = `{"type":"system","subtype":"init","session_id":"cli-session-1"}
{"type":"assistant","message":{"content":[{"type":"text","text":"Reading the repo."}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"go test ./..."}}]}}
{"type":"result","subtype":"success","is_error":false,"result":"%s","session_id":"cli-session-1"}
`

// resultLine is a Claude stream whose final answer is the given text.
func resultLine(finalText string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(finalText)
	return fmt.Sprintf(claudeSuccess, escaped)
}

const prJSON = `Finished. {"pullRequestTitle": "GEN-1 Speed up the API", "pullRequestBody": "Faster orders"}`

// commitScript plays a CLI that commits a change to the workspace and ends with the pull request JSON.
func commitScript(finalText string) sandboxtest.Script {
	return func(_ context.Context, _ int, spec sandbox.Spec, output io.Writer) (sandbox.Result, error) {
		if err := os.WriteFile(filepath.Join(spec.Workspace, "orders.go"), []byte("package api\n"), 0o644); err != nil {
			return sandbox.Result{}, err
		}
		for _, step := range [][]string{{"add", "orders.go"}, {"commit", "-m", "GEN-1 speed up orders"}} {
			if err := runGit(spec.Workspace, step...); err != nil {
				return sandbox.Result{}, err
			}
		}
		_, err := io.WriteString(output, resultLine(finalText))
		return sandbox.Result{}, err
	}
}
