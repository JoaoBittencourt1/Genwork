package commands

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func updaterEnvironment(fixture *updateFixture, address string) map[string]string {
	return map[string]string{
		"GENWORK_HOME":           fixture.workspace.Root,
		"UPDATER_HTTP_ADDR":      address,
		"GENWORK_UPDATER_SECRET": "kept-updater-secret-0123456789",
		"GATEWAY_INTERNAL_URL":   fixture.gateway.server.URL,
	}
}

func TestUpdaterNeedsItsConfiguration(t *testing.T) {
	fixture := newUpdateFixture(t)

	err, _ := executeWithEnv(t, fixture.workspace, fixture.runner, map[string]string{"GENWORK_HOME": fixture.workspace.Root}, "updater")

	for _, variable := range []string{"UPDATER_HTTP_ADDR", "GENWORK_UPDATER_SECRET", "GATEWAY_INTERNAL_URL"} {
		if err == nil || !strings.Contains(err.Error(), variable) {
			t.Errorf("error = %v, want it to name %s", err, variable)
		}
	}
}

func TestUpdaterRefusesAShortSecret(t *testing.T) {
	fixture := newUpdateFixture(t)
	environment := updaterEnvironment(fixture, "127.0.0.1:0")
	environment["GENWORK_UPDATER_SECRET"] = "short"

	err, _ := executeWithEnv(t, fixture.workspace, fixture.runner, environment, "updater")

	if err == nil || !strings.Contains(err.Error(), "GENWORK_UPDATER_SECRET") || strings.Contains(err.Error(), "short") {
		t.Errorf("error = %v", err)
	}
}

func freeAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().String()
}

func TestUpdaterServesTheStatusUntilStopped(t *testing.T) {
	fixture := newUpdateFixture(t)
	address := freeAddress(t)
	environment := updaterEnvironment(fixture, address)
	dependencies := Dependencies{
		Runner: fixture.runner, Prompter: &fakePrompter{}, IsInteractive: func() bool { return false },
		WorkingDir: func() (string, error) { return t.TempDir(), nil }, Getenv: func(name string) string { return environment[name] },
	}
	root := NewRootCommand(dependencies)
	root.SetArgs([]string{"updater"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan error, 1)
	go func() { stopped <- root.ExecuteContext(ctx) }()

	var status int
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		request, _ := http.NewRequest(http.MethodGet, "http://"+address+"/v1/status", nil)
		request.Header.Set("Authorization", "Bearer kept-updater-secret-0123456789")
		if response, err := http.DefaultClient.Do(request); err == nil {
			status = response.StatusCode
			response.Body.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()

	if status != http.StatusOK {
		t.Errorf("status = %d", status)
	}
	select {
	case err := <-stopped:
		if err != nil {
			t.Errorf("updater = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the updater did not stop")
	}
}

func TestUpdaterReplaceRecreatesTheUpdaterWithTheInstalledVersion(t *testing.T) {
	fixture := newUpdateFixture(t)

	err, output := executeWithEnv(t, fixture.workspace, fixture.runner, map[string]string{"GENWORK_HOME": fixture.workspace.Root}, "updater", "replace")

	if err != nil {
		t.Fatalf("error: %v (%s)", err, output)
	}
	runs := fixture.runsMatching("up -d --no-deps --force-recreate updater")
	if len(runs) != 1 || runs[0].version() != "0.5.0" {
		t.Errorf("runs = %+v", fixture.runner.runs)
	}
}

func TestTheUpdaterReplacesItselfFromAOneOffContainer(t *testing.T) {
	fixture := newUpdateFixture(t)
	installation := newWorkspaceInstallation(Dependencies{Runner: fixture.runner}, fixture.workspace, &bytes.Buffer{}, &bytes.Buffer{}, fixture.gateway.server.URL)
	installation.insideUpdater = true

	if err := installation.replaceUpdater(context.Background(), "0.5.2"); err != nil {
		t.Fatal(err)
	}

	runs := fixture.runsMatching("run -d --rm --no-deps updater genwork updater replace")
	if len(runs) != 1 || runs[0].version() != "0.5.2" {
		t.Errorf("runs = %q", fixture.runner.commandLines())
	}
}
