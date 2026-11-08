package command_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/adapters"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/claims"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/command"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/config"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/genagentstest"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/sandbox"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/sandboxtest"
)

func envFrom(values map[string]string) config.Getenv {
	return func(name string) string { return values[name] }
}

func registeredEnv(t *testing.T, fake *genagentstest.FakeGenagents, extra map[string]string) (string, config.Getenv) {
	t.Helper()
	home := t.TempDir()
	if err := config.Save(home, config.Stored{InstanceURL: fake.URL(), RunnerToken: "runner-token", RunnerSecret: genagentstest.Secret}); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{"GENWORK_RUNNER_HOME": home, "GENWORK_IMAGE_REGISTRY": "registry.example"}
	for key, value := range extra {
		values[key] = value
	}
	return home, envFrom(values)
}

func TestVersionDefaultsToTheDevelopmentBuild(t *testing.T) {
	var output bytes.Buffer

	if err := command.Run(context.Background(), []string{"version"}, envFrom(nil), &output); err != nil {
		t.Fatalf("version: %v", err)
	}

	if strings.TrimSpace(output.String()) != "0.0.0-dev" || command.Version != "0.0.0-dev" {
		t.Errorf("version = %q", output.String())
	}
}

func TestUnknownCommandsPrintTheUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"launch"}, {"account"}, {"account", "logout"}} {
		err := command.Run(context.Background(), args, envFrom(nil), io.Discard)
		if err == nil || !strings.Contains(err.Error(), "usage") {
			t.Errorf("%v: error = %v", args, err)
		}
	}
}

func TestRegisterCommandStoresTheSecret(t *testing.T) {
	fake := genagentstest.NewFakeGenagents(t)
	home := t.TempDir()
	var output bytes.Buffer

	err := command.Run(context.Background(), []string{"register", "--instance", fake.URL(), "--token", "registration-token"},
		envFrom(map[string]string{"GENWORK_RUNNER_HOME": home}), &output)

	if err != nil {
		t.Fatalf("register: %v", err)
	}
	stored, _ := config.LoadStored(home)
	if stored.RunnerSecret != genagentstest.Secret || !strings.Contains(output.String(), "owner-only") {
		t.Errorf("stored = %+v, output = %q", stored, output.String())
	}
	if strings.Contains(output.String(), genagentstest.Secret) {
		t.Error("the secret was printed")
	}
}

func TestRegisterCommandUsesTheEnvironmentAndRejectsMissingInput(t *testing.T) {
	fake := genagentstest.NewFakeGenagents(t)
	home := t.TempDir()
	env := envFrom(map[string]string{"GENWORK_RUNNER_HOME": home, "GENWORK_INSTANCE_URL": fake.URL(), "GENWORK_REGISTRATION_TOKEN": "registration-token"})

	if err := command.Run(context.Background(), []string{"register"}, env, io.Discard); err != nil {
		t.Fatalf("register from the environment: %v", err)
	}
	if err := command.Run(context.Background(), []string{"register"}, envFrom(map[string]string{"GENWORK_RUNNER_HOME": t.TempDir()}), io.Discard); err == nil {
		t.Error("register without input succeeded")
	}
	if err := command.Run(context.Background(), []string{"register", "--bogus"}, env, io.Discard); err == nil {
		t.Error("an unknown flag was accepted")
	}
}

func TestRunWithoutRegistrationExplainsWhatToDo(t *testing.T) {
	err := command.Run(context.Background(), []string{"run", "--directory-mode"}, envFrom(map[string]string{"GENWORK_RUNNER_HOME": t.TempDir()}), io.Discard)

	if !errors.Is(err, config.ErrNotRegistered) {
		t.Errorf("error = %v", err)
	}
}

func TestRunRefusesToStartWithoutAContainerEngineUnlessDirectoryMode(t *testing.T) {
	fake := genagentstest.NewFakeGenagents(t)
	_, env := registeredEnv(t, fake, map[string]string{"GENWORK_CONTAINER_SOCKET": "/nonexistent/engine.sock", "XDG_RUNTIME_DIR": t.TempDir(), "DOCKER_HOST": ""})

	err := command.Serve(context.Background(), env, command.ServeOptions{})

	if !errors.Is(err, sandbox.ErrNoEngine) || !strings.Contains(err.Error(), "directory-mode") {
		t.Errorf("error = %v", err)
	}
}

func TestServeNeedsAnImageRegistryOrACustomImage(t *testing.T) {
	fake := genagentstest.NewFakeGenagents(t)
	_, env := registeredEnv(t, fake, map[string]string{"GENWORK_IMAGE_REGISTRY": ""})
	launcher := sandboxtest.NewFakeLauncher(nil)

	err := command.Serve(context.Background(), env, command.ServeOptions{Launcher: launcher})

	if err == nil || !strings.Contains(err.Error(), "GENWORK_IMAGE_REGISTRY") {
		t.Errorf("error = %v", err)
	}
}

func TestServeClaimsAndExecutesARunUntilTheContextEnds(t *testing.T) {
	fake := genagentstest.NewFakeGenagents(t)
	home, env := registeredEnv(t, fake, nil)
	launcher := sandboxtest.NewFakeLauncher(func(_ context.Context, _ int, _ sandbox.Spec, output io.Writer) (sandbox.Result, error) {
		_, err := io.WriteString(output, `{"type":"result","subtype":"success","result":"You have 3 tasks.","session_id":"cli-1"}`+"\n")
		return sandbox.Result{}, err
	})
	fake.QueueRun(genagentstest.ChatRun("run-1"))
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan error, 1)
	go func() {
		finished <- command.Serve(ctx, env, command.ServeOptions{Launcher: launcher, HeartbeatInterval: 20 * time.Millisecond, PollInterval: 20 * time.Millisecond})
	}()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && !hasResult(fake, "run-1") {
		time.Sleep(20 * time.Millisecond)
	}
	cancel()

	if err := <-finished; err != nil {
		t.Fatalf("serve: %v", err)
	}
	if !hasResult(fake, "run-1") || fake.HeartbeatCount() == 0 {
		t.Errorf("events = %v, heartbeats = %d", fake.EventsOf("run-1"), fake.HeartbeatCount())
	}
	if entries, _ := os.ReadDir(filepath.Join(home, "runs")); len(entries) != 0 {
		t.Errorf("workspaces left behind: %v", entries)
	}
}

func hasResult(fake *genagentstest.FakeGenagents, runToken string) bool {
	for _, event := range fake.EventsOf(runToken) {
		if event.Type == "result" {
			return true
		}
	}
	return false
}

func TestServeStopsWithErrRevokedWhenTheSecretIsRefused(t *testing.T) {
	fake := genagentstest.NewFakeGenagents(t)
	_, env := registeredEnv(t, fake, nil)
	fake.Revoke()

	err := command.Serve(context.Background(), env, command.ServeOptions{Launcher: sandboxtest.NewFakeLauncher(nil), HeartbeatInterval: 20 * time.Millisecond})

	if !errors.Is(err, claims.ErrRevoked) {
		t.Errorf("error = %v", err)
	}
}

func TestStartupRemovesLeftoversAndReportsThoseRunsAsLost(t *testing.T) {
	fake := genagentstest.NewFakeGenagents(t)
	home, env := registeredEnv(t, fake, nil)
	if err := os.MkdirAll(filepath.Join(home, "runs", "old-run"), 0o700); err != nil {
		t.Fatal(err)
	}
	launcher := sandboxtest.NewFakeLauncher(nil)
	launcher.Leftovers = []string{"old-run"}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	err := command.Serve(ctx, env, command.ServeOptions{Launcher: launcher, HeartbeatInterval: 50 * time.Millisecond})

	if err != nil {
		t.Fatalf("serve: %v", err)
	}
	events := fake.EventsOf("old-run")
	if len(events) != 1 || events[0].Type != "error" || events[0].Payload["reason"] != "runner_lost" {
		t.Errorf("events = %+v", events)
	}
	if _, err := os.Stat(filepath.Join(home, "runs", "old-run")); !os.IsNotExist(err) {
		t.Error("the leftover workspace survived")
	}
}

func TestLoginContainerIsolatesTheSignInToOneProfile(t *testing.T) {
	adapter, _ := adapters.For("anthropic")
	settings := config.Config{SandboxImage: "registry.example/genwork-sandbox:2.0.0", EngineSocket: "/run/podman/podman.sock"}
	directory, err := os.MkdirTemp("", "podman")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(directory)
	socket := filepath.Join(directory, "podman.sock")
	listener := listen(t, socket)
	defer listener.Close()
	settings.EngineSocket = socket

	cmd, err := command.BuildLoginCommand(adapter, "/data/profiles/acct-1", settings, envFrom(nil), false)

	if err != nil {
		t.Fatalf("build: %v", err)
	}
	joined := strings.Join(cmd.Args, " ")
	for _, want := range []string{"podman run --rm -it", "--cap-drop ALL", "--security-opt no-new-privileges", "--read-only", "-v /data/profiles/acct-1:/home/sandbox/.claude:rw", "CLAUDE_CONFIG_DIR=/home/sandbox/.claude", "registry.example/genwork-sandbox:2.0.0 claude auth login"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the login command lacks %q: %s", want, joined)
		}
	}
	if strings.Contains(joined, "docker.sock") || strings.Contains(joined, "podman.sock") || strings.Count(joined, "-v ") != 1 {
		t.Errorf("the login mounts more than the profile: %s", joined)
	}
}

func TestLoginInDirectoryModePointsTheCLIAtTheHostProfile(t *testing.T) {
	adapter, _ := adapters.For("openai")

	cmd, err := command.BuildLoginCommand(adapter, "/data/profiles/acct-2", config.Config{}, envFrom(nil), true)

	if err != nil || cmd.Args[0] != "codex" || cmd.Args[1] != "login" {
		t.Fatalf("command = %v, err = %v", cmd, err)
	}
	if !contains(cmd.Env, "CODEX_HOME=/data/profiles/acct-2") {
		t.Errorf("env lacks the host profile: %v", cmd.Env[len(cmd.Env)-3:])
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

func TestAccountLoginNeedsAnAccountThisRunnerHolds(t *testing.T) {
	fake := genagentstest.NewFakeGenagents(t)
	_, env := registeredEnv(t, fake, nil)

	err := command.Run(context.Background(), []string{"account", "login", "--directory-mode", "unknown-account"}, env, io.Discard)

	if err == nil || !strings.Contains(err.Error(), "holds no account") {
		t.Errorf("error = %v", err)
	}
	if err := command.Run(context.Background(), []string{"account", "login"}, env, io.Discard); err == nil {
		t.Error("login without an account succeeded")
	}
}

func TestAccountLoginReportsASuccessfulSignIn(t *testing.T) {
	fake := genagentstest.NewFakeGenagents(t)
	fake.AssignAccount("acct-1", "xai")
	_, env := registeredEnv(t, fake, nil)

	err := command.Run(context.Background(), []string{"account", "login", "--directory-mode", "acct-1"}, env, io.Discard)

	if err == nil || !strings.Contains(err.Error(), "no subscription sign-in") {
		t.Errorf("error = %v", err)
	}
}
