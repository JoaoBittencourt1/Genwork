// Package execution runs one claimed run end to end: workspace, sandbox, CLI turns, events, and the host-side push, pull
// request, and review posting.
package execution

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/adapters"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/api"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/sandbox"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/workspace"
)

const (
	defaultPollInterval = 3 * time.Second
	defaultTokenRefresh = 10 * time.Minute
	outputTailBytes     = 4000
)

// Failure reasons a runner reports; genagents knows the same set.
const (
	reasonTimeout          = "timeout"
	reasonSandboxLimit     = "sandbox_limit"
	reasonAdapterError     = "adapter_error"
	reasonGithubPostFailed = "github_post_failed"
	reasonAccountError     = "account_error"
	reasonRunFailed        = "run_failed"
	reasonGithubMissing    = "github_credentials_missing"

	// githubTokenMargin renews the platform's GitHub token this long before it expires.
	githubTokenMargin = 15 * time.Minute
)

// GithubPoster is what the executor needs from GitHub (rule 9).
type GithubPoster interface {
	DefaultBranch(ctx context.Context, repository string) (string, error)
	CreatePullRequest(ctx context.Context, repository, head, base, title, description string) (PullRequestCreated, error)
	CreateReview(ctx context.Context, repository string, number int, summary string, comments []ReviewComment) (string, error)
	// WithToken is the same poster authenticating with a run's own token.
	WithToken(token string) GithubPoster
}

type PullRequestCreated struct {
	Number int
	URL    string
}

type ReviewComment struct {
	Path string
	Line int
	Body string
}

type Settings struct {
	SandboxImage string
	Limits       sandbox.Limits
	// ProfileDir resolves an account's isolated sign-in profile directory.
	ProfileDir   func(accountToken string) string
	PollInterval time.Duration
	TokenRefresh time.Duration
	Sleep        func(time.Duration)
	// TimeoutUnit is what a run's timeout is counted in; it is a minute, and tests shorten it.
	TimeoutUnit time.Duration
}

type Executor struct {
	api        RunAPI
	launcher   sandbox.Launcher
	workspaces *workspace.Manager
	github     GithubPoster
	settings   Settings
}

func NewExecutor(runAPI RunAPI, launcher sandbox.Launcher, workspaces *workspace.Manager, github GithubPoster, settings Settings) *Executor {
	if settings.PollInterval == 0 {
		settings.PollInterval = defaultPollInterval
	}
	if settings.TokenRefresh == 0 {
		settings.TokenRefresh = defaultTokenRefresh
	}
	if settings.TimeoutUnit == 0 {
		settings.TimeoutUnit = time.Minute
	}
	if settings.Sleep == nil {
		settings.Sleep = time.Sleep
	}
	return &Executor{api: runAPI, launcher: launcher, workspaces: workspaces, github: github, settings: settings}
}

// execution is the state of one run while it executes.
type execution struct {
	executor  *Executor
	run       api.Run
	adapter   adapters.Adapter
	workspace *workspace.Workspace
	poster    *poster
	// github posts with the run's token from the platform, or the runner's own when the platform issued none.
	github          GithubPoster
	githubExpiresAt time.Time
	profileDir      string
	secrets         []string
	// repositories maps "owner/name" to its directory in the workspace.
	repositories map[string]string
	branch       string
	cancel       context.CancelCauseFunc
	tokenMutex   sync.Mutex
}

var (
	errRunEnded                 = errors.New("the run ended elsewhere")
	errGithubCredentialsMissing = errors.New("no GitHub credentials: connect the GitHub App in Tasks → GitHub and install it on the repository, or set GH_TOKEN on the runner")
)

// Execute runs a claimed run and always cleans up: the workspace, with every secret file in it, is removed in every end
// state, and the sandbox containers are removed as each turn ends.
func (executor *Executor) Execute(ctx context.Context, run api.Run) {
	adapter, err := adapters.For(run.Account.Provider)
	if err != nil {
		poster := newPoster(executor.api, run.RunToken, executor.settings.Sleep)
		poster.Post(ctx, adapters.EventError, map[string]any{"reason": reasonAdapterError, "message": err.Error()})
		return
	}
	runCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	current := &execution{
		executor: executor, run: run, adapter: adapter, poster: newPoster(executor.api, run.RunToken, executor.settings.Sleep),
		secrets: []string{run.Account.APIKey}, repositories: map[string]string{}, cancel: cancel, github: executor.github,
	}
	defer current.cleanup()
	go current.watchForCancellation(runCtx)
	current.drive(runCtx)
}

func (current *execution) cleanup() {
	if current.workspace == nil {
		return
	}
	if err := current.adapter.Finish(current.workspace.Dir, current.profileDir); err != nil {
		slog.Warn("finish the adapter", "run", current.run.RunToken, "err", err)
	}
	if err := current.workspace.Remove(); err != nil {
		slog.Warn("remove the workspace", "run", current.run.RunToken, "err", err)
	}
}

// watchForCancellation stops the run when genagents says it ended: cancelled by a person, failed by the sweeper, or
// finished. The same poll also keeps the run's events flowing back its state.
func (current *execution) watchForCancellation(ctx context.Context) {
	ticker := time.NewTicker(current.executor.settings.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			state, err := current.executor.api.RunState(ctx, current.run.RunToken)
			if err == nil && !isActive(state) {
				current.cancel(errRunEnded)
				return
			}
		}
	}
}

func isActive(state string) bool {
	return state == "queued" || state == "running" || state == "waiting_for_input"
}

func (current *execution) fail(ctx context.Context, reason, message string, extra map[string]any) {
	payload := map[string]any{"reason": reason, "message": adapters.Redact(message, current.secrets...)}
	for key, value := range extra {
		payload[key] = value
	}
	current.poster.Post(context.WithoutCancel(ctx), adapters.EventError, payload)
}

// drive is the run's whole story: set up, take turns, finish.
func (current *execution) drive(ctx context.Context) {
	if err := current.setUp(ctx); err != nil {
		reason := reasonAdapterError
		if errors.Is(err, errGithubCredentialsMissing) {
			reason = reasonGithubMissing
		}
		if ctx.Err() == nil {
			current.fail(ctx, reason, err.Error(), nil)
		}
		return
	}
	prompt := current.promptWithEnvironment()
	session := ""
	if current.run.ResumeSession != nil {
		session = *current.run.ResumeSession
	}
	for {
		outcome, ended := current.takeTurn(ctx, prompt, session)
		if ended {
			return
		}
		if outcome.SessionID != "" {
			session = outcome.SessionID
		}
		if messages, found := current.pendingMessages(ctx); found {
			prompt = messages
			current.poster.Post(ctx, adapters.EventStatus, map[string]any{"state": "running", "sessionId": session})
			continue
		}
		next, finished := current.finish(ctx, outcome, session)
		if finished {
			return
		}
		prompt = next
	}
}

// setUp creates the workspace, clones on the host, writes the CLI's files, and starts renewing the Genwork token.
func (current *execution) setUp(ctx context.Context) error {
	var err error
	if current.workspace, err = current.executor.workspaces.Create(current.run.RunToken); err != nil {
		return err
	}
	if err := current.acquireGithubCredentials(ctx); err != nil {
		return err
	}
	if err := current.checkOutRepositories(ctx); err != nil {
		return err
	}
	if current.run.Account.Mode == "subscription" {
		current.profileDir = current.executor.settings.ProfileDir(current.run.Account.Token)
		if err := os.MkdirAll(current.profileDir, 0o700); err != nil {
			return fmt.Errorf("create the account profile: %w", err)
		}
	}
	if err := current.refreshToken(ctx); err != nil {
		return fmt.Errorf("get a Genwork token: %w", err)
	}
	go current.keepTokenFresh(ctx)
	return nil
}

func (current *execution) checkOutRepositories(ctx context.Context) error {
	switch current.run.Kind {
	case "task":
		directory, err := current.workspace.Clone(ctx, current.run.Repository, "")
		if err != nil {
			return err
		}
		current.repositories[current.run.Repository], current.branch = directory, current.run.Branch
		return current.workspace.PrepareBranch(ctx, directory, current.run.Branch, current.run.Bot.Name, botEmail(current.run.Bot.Name))
	case "review":
		return current.checkOutPullRequests(ctx)
	}
	return nil
}

func botEmail(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(name), "-")) + "@bots.genwork.invalid"
}

func (current *execution) checkOutPullRequests(ctx context.Context) error {
	for _, pullRequest := range current.run.PullRequests {
		directory, cloned := current.repositories[pullRequest.Repository]
		if !cloned {
			var err error
			relative := "repos/" + strings.ReplaceAll(pullRequest.Repository, "/", "__")
			if directory, err = current.workspace.Clone(ctx, pullRequest.Repository, relative); err != nil {
				return err
			}
			current.repositories[pullRequest.Repository] = directory
		}
		if _, err := current.workspace.CheckoutPullRequest(ctx, directory, pullRequest.Number); err != nil {
			return err
		}
	}
	return nil
}

// workingDirectory is where the CLI runs and where the runner writes its files: the clone for a task run, the workspace
// itself otherwise.
func (current *execution) workingDirectory() string {
	if directory, found := current.repositories[current.run.Repository]; found && current.run.Kind == "task" {
		return directory
	}
	return current.workspace.Dir
}

func (current *execution) promptWithEnvironment() string {
	switch current.run.Kind {
	case "review":
		var note strings.Builder
		note.WriteString("\n\nThe pull requests are checked out for you, each head on a branch named pr-<number>, in these directories under " + sandbox.WorkspaceMount + ":\n")
		for repository, directory := range current.repositories {
			relative, _ := filepath.Rel(current.workspace.Dir, directory)
			fmt.Fprintf(&note, "- %s: %s/%s (compare with origin/HEAD)\n", repository, sandbox.WorkspaceMount, relative)
		}
		return current.run.Prompt + note.String()
	}
	return current.run.Prompt
}

func (current *execution) refreshToken(ctx context.Context) error {
	current.tokenMutex.Lock()
	defer current.tokenMutex.Unlock()
	token, err := current.executor.api.GenworkToken(ctx, current.run.RunToken)
	if err != nil {
		return err
	}
	current.secrets = append(current.secrets, token.AccessToken)
	return current.adapter.Prepare(current.workingDirectory(), current.profileDir, current.run, token)
}

// keepTokenFresh rewrites the CLI's MCP configuration with a new token before the old one expires; the sandbox cannot
// call the runner, so the runner does it.
func (current *execution) keepTokenFresh(ctx context.Context) {
	ticker := time.NewTicker(current.executor.settings.TokenRefresh)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := current.refreshToken(ctx); err != nil && ctx.Err() == nil {
				slog.Warn("renew the Genwork token", "run", current.run.RunToken, "err", err)
			}
			current.renewGithubToken(ctx)
		}
	}
}

// takeTurn runs the CLI once and reports its events. It returns ended when the run is over: failed, timed out, or ended
// elsewhere.
func (current *execution) takeTurn(ctx context.Context, prompt, session string) (adapters.Outcome, bool) {
	invocation := current.adapter.Invocation(current.run, prompt, session)
	spec := sandbox.Spec{
		RunToken: current.run.RunToken, Image: current.executor.settings.SandboxImage, Workspace: current.workspace.Dir,
		Env: invocation.Env, Command: invocation.Command, Limits: current.executor.settings.Limits,
	}
	if invocation.Profile != nil {
		spec.Profile = &sandbox.Profile{Dir: current.profileDir, Mount: invocation.Profile.Mount}
	}
	parser := current.adapter.NewParser(current.secrets)
	writer := newEventWriter(ctx, parser, current.poster)
	turnCtx, stopTurn := context.WithTimeout(ctx, time.Duration(current.run.TimeoutMinutes)*current.executor.settings.TimeoutUnit)
	defer stopTurn()
	result, err := current.executor.launcher.Run(turnCtx, spec, writer)
	writer.Flush()
	outcome := parser.Outcome()
	return outcome, current.turnEnded(ctx, turnCtx, result, err, outcome, writer.Tail())
}

// turnEnded reports a failed turn and says whether the run is over.
func (current *execution) turnEnded(ctx, turnCtx context.Context, result sandbox.Result, err error, outcome adapters.Outcome, tail string) bool {
	switch {
	case errors.Is(context.Cause(ctx), errRunEnded) || ctx.Err() != nil:
		return true
	case errors.Is(turnCtx.Err(), context.DeadlineExceeded):
		current.stopSandbox()
		current.fail(ctx, reasonTimeout, "the run exceeded its timeout", nil)
		return true
	case err != nil:
		current.fail(ctx, reasonAdapterError, err.Error(), nil)
		return true
	case result.OOMKilled:
		current.fail(ctx, reasonSandboxLimit, "the sandbox exceeded its memory limit", map[string]any{"limit": "memory"})
		return true
	case outcome.Failed && outcome.AuthFailure:
		current.fail(ctx, reasonAccountError, outcome.ErrorMessage, map[string]any{"authFailure": true})
		return true
	case outcome.Failed:
		current.fail(ctx, reasonRunFailed, outcome.ErrorMessage, nil)
		return true
	case result.ExitCode != 0:
		current.fail(ctx, reasonAdapterError, fmt.Sprintf("the CLI exited with code %d: %s", result.ExitCode, tail), nil)
		return true
	}
	return false
}

func (current *execution) stopSandbox() {
	stopCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := current.executor.launcher.Stop(stopCtx, current.run.RunToken); err != nil {
		slog.Warn("stop the sandbox", "run", current.run.RunToken, "err", err)
	}
}

// pendingMessages fetches people's new messages and acknowledges them; they are delivered at the turn boundary by
// resuming the session.
func (current *execution) pendingMessages(ctx context.Context) (string, bool) {
	messages, err := current.executor.api.ListMessages(ctx, current.run.RunToken)
	if err != nil || len(messages) == 0 {
		return "", false
	}
	texts := make([]string, 0, len(messages))
	for _, message := range messages {
		texts = append(texts, message.Text)
	}
	if err := current.executor.api.AcknowledgeMessages(ctx, current.run.RunToken, messages[len(messages)-1].Sequence); err != nil {
		slog.Warn("acknowledge messages", "run", current.run.RunToken, "err", err)
	}
	return strings.Join(texts, "\n\n"), true
}

// eventWriter splits the sandbox output into lines, feeds each to the parser, and posts the events it yields.
type eventWriter struct {
	ctx    context.Context
	parser adapters.Parser
	poster *poster
	buffer bytes.Buffer
	tail   []byte
}

func newEventWriter(ctx context.Context, parser adapters.Parser, poster *poster) *eventWriter {
	return &eventWriter{ctx: ctx, parser: parser, poster: poster}
}

var _ io.Writer = (*eventWriter)(nil)

func (writer *eventWriter) Write(data []byte) (int, error) {
	writer.tail = append(writer.tail, data...)
	if len(writer.tail) > outputTailBytes {
		writer.tail = writer.tail[len(writer.tail)-outputTailBytes:]
	}
	writer.buffer.Write(data)
	for {
		line, err := writer.buffer.ReadBytes('\n')
		if err != nil {
			writer.buffer.Reset()
			writer.buffer.Write(line)
			return len(data), nil
		}
		writer.emit(line)
	}
}

// Flush handles a last line that had no newline.
func (writer *eventWriter) Flush() {
	if writer.buffer.Len() > 0 {
		writer.emit(writer.buffer.Bytes())
		writer.buffer.Reset()
	}
}

func (writer *eventWriter) emit(line []byte) {
	for _, event := range writer.parser.Feed(line) {
		writer.poster.Post(writer.ctx, event.Type, event.Payload)
	}
}

func (writer *eventWriter) Tail() string {
	return strings.TrimSpace(string(writer.tail))
}

func needsGithub(kind string) bool {
	return kind == "task" || kind == "review"
}

// acquireGithubCredentials prefers a token the platform issues for this run's repositories and falls back to the runner's
// own; a run that needs GitHub fails when neither exists.
func (current *execution) acquireGithubCredentials(ctx context.Context) error {
	if !needsGithub(current.run.Kind) {
		return nil
	}
	if current.useGithubTokenFromPlatform(ctx) {
		return nil
	}
	if !current.executor.workspaces.HasToken() {
		return errGithubCredentialsMissing
	}
	return nil
}

func (current *execution) useGithubTokenFromPlatform(ctx context.Context) bool {
	token, err := current.executor.api.GithubToken(ctx, current.run.RunToken)
	if err != nil || token.Token == "" {
		if err != nil {
			slog.Info("the platform issued no GitHub token; using the runner's", "run", current.run.RunToken, "err", err)
		}
		return false
	}
	current.tokenMutex.Lock()
	defer current.tokenMutex.Unlock()
	current.secrets = append(current.secrets, token.Token)
	current.workspace.UseToken(token.Token)
	current.github = current.executor.github.WithToken(token.Token)
	current.githubExpiresAt = token.ExpiresAt
	return true
}

// renewGithubToken replaces the platform's token before it expires, so a long run can still push and post.
func (current *execution) renewGithubToken(ctx context.Context) {
	current.tokenMutex.Lock()
	expiresAt := current.githubExpiresAt
	current.tokenMutex.Unlock()
	if expiresAt.IsZero() || time.Until(expiresAt) > githubTokenMargin {
		return
	}
	if !current.useGithubTokenFromPlatform(ctx) && ctx.Err() == nil {
		slog.Warn("renew the GitHub token", "run", current.run.RunToken)
	}
}

func (current *execution) githubPoster() GithubPoster {
	current.tokenMutex.Lock()
	defer current.tokenMutex.Unlock()
	return current.github
}
