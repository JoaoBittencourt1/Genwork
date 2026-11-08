package signin_test

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/api"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/genagentstest"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/sandbox"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/sandboxtest"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/signin"
)

const claudeLoginURL = "https://claude.ai/oauth/authorize?code=true&client_id=fixture&state=fixture-state"

// claudeLogin plays `claude auth login`: it prints the URL, waits for the pasted code, and exits 0 when it is right.
func claudeLogin(ctx context.Context, terminal io.Writer, typed *sandboxtest.TypedInput) (sandbox.Result, error) {
	_, _ = io.WriteString(terminal, "Browser didn't open? Use the url below to sign in:\r\n\r\n"+claudeLoginURL+"\r\n\r\nPaste code here if prompted > ")
	code, ok := typed.ReadLine(ctx)
	if !ok {
		return sandbox.Result{ExitCode: 137}, nil
	}
	if code != "anthropic-code#state" {
		_, _ = io.WriteString(terminal, "\r\nOAuth error: Invalid code\r\n")
		return sandbox.Result{ExitCode: 1}, nil
	}
	_, _ = io.WriteString(terminal, "\r\nLogin successful.\r\n")
	return sandbox.Result{}, nil
}

type harness struct {
	t        *testing.T
	fake     *genagentstest.FakeGenagents
	launcher *sandboxtest.FakeInteractiveLauncher
	loop     *signin.Loop
	root     string
}

func newHarness(t *testing.T, script sandboxtest.TerminalScript, mutate func(*signin.Settings)) *harness {
	t.Helper()
	h := &harness{t: t, fake: genagentstest.NewFakeGenagents(t), launcher: sandboxtest.NewFakeInteractiveLauncher(script), root: t.TempDir()}
	settings := signin.Settings{
		Image: "registry.example/genwork-sandbox:2.2.0", ProfileDir: func(account string) string { return filepath.Join(h.root, "profiles", account) },
		ScratchDir: h.root, URLTimeout: 5 * time.Second, VerifyTimeout: 5 * time.Second,
	}
	if mutate != nil {
		mutate(&settings)
	}
	h.loop = signin.NewLoop(api.NewClient(h.fake.URL(), genagentstest.Secret, nil), h.launcher, settings)
	return h
}

func signInOf(token, provider, state string) map[string]any {
	return map[string]any{
		"token": token, "accountToken": "acct-" + token, "provider": provider, "state": state,
		"expiresAt": time.Now().Add(10 * time.Minute).UTC().Format(time.RFC3339),
	}
}

func (h *harness) poll() {
	h.t.Helper()
	if err := h.loop.Poll(context.Background()); err != nil {
		h.t.Fatalf("poll: %v", err)
	}
}

// waitForReport waits until the runner reported the state for the sign-in, and returns that report.
func (h *harness) waitForReport(token, state string) map[string]any {
	h.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		for _, report := range h.fake.Reports() {
			if report.Token == token && report.Payload["state"] == state {
				return report.Payload
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	h.t.Fatalf("no %s report for %s; reports = %+v", state, token, h.fake.Reports())
	return nil
}

func TestClaudeSignInRelaysTheURLTypesTheCodeAndSucceeds(t *testing.T) {
	h := newHarness(t, claudeLogin, nil)
	h.fake.SetSignIns(signInOf("si-1", "anthropic", "requested"))

	h.poll()
	prompt := h.waitForReport("si-1", "awaiting_user")
	if prompt["url"] != claudeLoginURL || prompt["expectsCode"] != true {
		t.Fatalf("prompt = %+v", prompt)
	}
	pasted := signInOf("si-1", "anthropic", "code_submitted")
	pasted["code"] = "anthropic-code#state"
	h.fake.SetSignIns(pasted)
	h.poll()

	h.waitForReport("si-1", "verifying")
	h.waitForReport("si-1", "succeeded")
	spec := h.launcher.SpecsSnapshot()[0]
	if spec.SignInToken != "si-1" || spec.RunToken != "" || spec.Profile == nil || spec.Profile.Mount != "/home/sandbox/.claude" {
		t.Errorf("spec = %+v", spec)
	}
	if spec.Profile.Dir != filepath.Join(h.root, "profiles", "acct-si-1") || spec.Env["CLAUDE_CONFIG_DIR"] != "/home/sandbox/.claude" {
		t.Errorf("the sign-in does not use the account's own profile: %+v", spec)
	}
	if len(spec.Command) != 3 || spec.Command[0] != "claude" || spec.Command[1] != "auth" || spec.Command[2] != "login" {
		t.Errorf("command = %v", spec.Command)
	}
}

func TestAWrongCodeFailsWithTheCLIMessageButNotTheCode(t *testing.T) {
	h := newHarness(t, claudeLogin, nil)
	h.fake.SetSignIns(signInOf("si-2", "anthropic", "requested"))
	h.poll()
	h.waitForReport("si-2", "awaiting_user")
	pasted := signInOf("si-2", "anthropic", "code_submitted")
	pasted["code"] = "wrong-code-value-123"
	h.fake.SetSignIns(pasted)
	h.poll()

	failure := h.waitForReport("si-2", "failed")

	message, _ := failure["message"].(string)
	if !strings.Contains(message, "Invalid code") || strings.Contains(message, "wrong-code-value-123") {
		t.Errorf("message = %q", message)
	}
}

func TestCodexDeviceSignInReportsTheUserCode(t *testing.T) {
	h := newHarness(t, func(ctx context.Context, terminal io.Writer, _ *sandboxtest.TypedInput) (sandbox.Result, error) {
		_, _ = io.WriteString(terminal, "Open https://auth.openai.com/codex/device\r\nEnter this one-time code: ABCD-12345\r\n")
		<-ctx.Done()
		return sandbox.Result{ExitCode: 137}, nil
	}, nil)
	h.fake.SetSignIns(signInOf("si-3", "openai", "requested"))

	h.poll()

	prompt := h.waitForReport("si-3", "awaiting_user")
	if prompt["url"] != "https://auth.openai.com/codex/device" || prompt["userCode"] != "ABCD-12345" || prompt["expectsCode"] != false {
		t.Errorf("prompt = %+v", prompt)
	}
	h.fake.SetSignIns()
	h.poll()
	waitFor(t, func() bool { return h.launcher.ClosedCount() == 1 })
}

func TestACancelledSignInStopsTheCLIWithoutAReport(t *testing.T) {
	h := newHarness(t, claudeLogin, nil)
	h.fake.SetSignIns(signInOf("si-4", "anthropic", "requested"))
	h.poll()
	h.waitForReport("si-4", "awaiting_user")

	h.fake.SetSignIns()
	h.poll()

	waitFor(t, func() bool { return h.launcher.ClosedCount() == 1 })
	time.Sleep(50 * time.Millisecond)
	for _, report := range h.fake.Reports() {
		if report.Payload["state"] == "failed" || report.Payload["state"] == "succeeded" {
			t.Errorf("a cancelled sign-in was reported: %+v", report)
		}
	}
}

func TestACLIThatPrintsNoURLFails(t *testing.T) {
	h := newHarness(t, func(ctx context.Context, terminal io.Writer, _ *sandboxtest.TypedInput) (sandbox.Result, error) {
		_, _ = io.WriteString(terminal, "Starting...\r\n")
		<-ctx.Done()
		return sandbox.Result{ExitCode: 137}, nil
	}, func(settings *signin.Settings) { settings.URLTimeout = 50 * time.Millisecond })
	h.fake.SetSignIns(signInOf("si-5", "cursor", "requested"))

	h.poll()

	failure := h.waitForReport("si-5", "failed")
	if message, _ := failure["message"].(string); !strings.Contains(message, "no sign-in URL") || !strings.Contains(message, "Starting...") {
		t.Errorf("message = %q", message)
	}
}

func TestAProviderWithoutSubscriptionSignInFails(t *testing.T) {
	h := newHarness(t, claudeLogin, nil)
	h.fake.SetSignIns(signInOf("si-6", "xai", "requested"))

	h.poll()

	h.waitForReport("si-6", "failed")
	if len(h.launcher.SpecsSnapshot()) != 0 {
		t.Error("a sandbox started for a provider without a sign-in")
	}
}

func TestASignInLeftWaitingByAnotherRunnerProcessFails(t *testing.T) {
	h := newHarness(t, claudeLogin, nil)
	h.fake.SetSignIns(signInOf("si-7", "anthropic", "awaiting_user"))

	h.poll()

	failure := h.waitForReport("si-7", "failed")
	if message, _ := failure["message"].(string); !strings.Contains(message, "restarted") {
		t.Errorf("message = %q", message)
	}
}

func TestAnExpiredSignInIsReportedAsFailed(t *testing.T) {
	h := newHarness(t, claudeLogin, nil)
	expiring := signInOf("si-8", "anthropic", "requested")
	expiring["expiresAt"] = time.Now().Add(150 * time.Millisecond).UTC().Format(time.RFC3339Nano)
	h.fake.SetSignIns(expiring)

	h.poll()

	failure := h.waitForReport("si-8", "failed")
	if message, _ := failure["message"].(string); !strings.Contains(message, "expired") {
		t.Errorf("message = %q", message)
	}
}

func TestRunStopsWhenTheContextEnds(t *testing.T) {
	h := newHarness(t, claudeLogin, func(settings *signin.Settings) { settings.PollInterval = 10 * time.Millisecond })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- h.loop.Run(ctx) }()
	h.fake.SetSignIns(signInOf("si-9", "anthropic", "requested"))
	h.waitForReport("si-9", "awaiting_user")

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("run: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the loop did not stop")
	}
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition never held")
}

// A runner that never executed a run has no workspaces directory yet; the sign-in creates it.
func TestASignInWorksOnARunnerThatNeverRan(t *testing.T) {
	scratch := filepath.Join(t.TempDir(), "never-created", "runs")
	h := newHarness(t, claudeLogin, func(settings *signin.Settings) { settings.ScratchDir = scratch })
	h.fake.SetSignIns(signInOf("si-10", "anthropic", "requested"))

	h.poll()

	h.waitForReport("si-10", "awaiting_user")
}
