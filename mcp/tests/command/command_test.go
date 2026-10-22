package command_test

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/auth"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/auth/authtest"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/command"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/command/commandtest"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform/apitest"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type commandFixture struct {
	api         *apitest.FakeAPI
	core        *authtest.FakeSessionCore
	store       *authtest.MemoryCredentialStore
	stdout      *bytes.Buffer
	stderr      *bytes.Buffer
	environment command.Environment
}

func newCommandFixture(t *testing.T, prompter command.Prompter) *commandFixture {
	api := apitest.NewFakeAPI(t)
	fixture := &commandFixture{
		api: api, core: authtest.NewFakeSessionCore(api), store: authtest.NewMemoryCredentialStore(),
		stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{},
	}
	fixture.environment = command.Environment{
		Getenv:          func(name string) string { return map[string]string{platform.InstanceURLVariable: api.URL()}[name] },
		Stdout:          fixture.stdout,
		Stderr:          fixture.stderr,
		Prompter:        prompter,
		CredentialStore: fixture.store,
		LockDirectory:   t.TempDir(),
		DeviceName:      "genwork-mcp on test",
		HTTPClient:      http.DefaultClient,
	}
	return fixture
}

func (fixture *commandFixture) run(args ...string) error {
	return command.Run(context.Background(), args, fixture.environment)
}

func (fixture *commandFixture) login(t *testing.T) {
	t.Helper()
	fixture.environment.Prompter = commandtest.NewScriptedPrompter(authtest.Password)
	if err := fixture.run("login", "--email", authtest.Email); err != nil {
		t.Fatalf("login: %v", err)
	}
}

func TestRunPrintsUsageAndVersion(t *testing.T) {
	fixture := newCommandFixture(t, nil)

	if err := fixture.run("help"); err != nil || !strings.Contains(fixture.stdout.String(), "genwork-mcp serve") {
		t.Fatalf("help: err = %v, stdout = %q", err, fixture.stdout)
	}
	fixture.stdout.Reset()
	if err := fixture.run("version"); err != nil || strings.TrimSpace(fixture.stdout.String()) != command.Version {
		t.Fatalf("version: err = %v, stdout = %q", err, fixture.stdout)
	}
}

func TestBuildWithoutLdflagsReportsTheDevelopmentVersion(t *testing.T) {
	if command.Version != "0.0.0-dev" {
		t.Errorf("Version = %q", command.Version)
	}
}

func TestRunRejectsAMissingOrUnknownCommand(t *testing.T) {
	fixture := newCommandFixture(t, nil)

	for _, args := range [][]string{nil, {"deploy"}} {
		if err := fixture.run(args...); !errors.Is(err, command.ErrUnknownCommand) {
			t.Fatalf("%v: err = %v", args, err)
		}
	}
	if !strings.Contains(fixture.stderr.String(), "Usage:") {
		t.Fatalf("stderr = %q", fixture.stderr)
	}
}

func TestLoginAsksForTheEmailAndPasswordThenStatusShowsTheUser(t *testing.T) {
	prompter := commandtest.NewScriptedPrompter(authtest.Email, authtest.Password)
	fixture := newCommandFixture(t, prompter)

	if err := fixture.run("login"); err != nil {
		t.Fatalf("login: %v", err)
	}
	fixture.stdout.Reset()
	if err := fixture.run("status"); err != nil {
		t.Fatalf("status: %v", err)
	}

	if !slices.Equal(prompter.Asked, []string{"E-mail: ", "Password: "}) {
		t.Fatalf("asked = %v", prompter.Asked)
	}
	if want := "Signed in to " + fixture.api.URL() + " as " + authtest.Email + ".\n"; fixture.stdout.String() != want {
		t.Fatalf("status = %q, want %q", fixture.stdout, want)
	}
}

func TestLoginWithTheEmailFlagOnlyAsksForThePassword(t *testing.T) {
	prompter := commandtest.NewScriptedPrompter(authtest.Password)
	fixture := newCommandFixture(t, prompter)

	if err := fixture.run("login", "--email", authtest.Email); err != nil {
		t.Fatalf("login: %v", err)
	}

	if !slices.Equal(prompter.Asked, []string{"Password: "}) {
		t.Fatalf("asked = %v", prompter.Asked)
	}
}

func TestLoginReportsBadFlagsPromptFailuresAndAWrongPassword(t *testing.T) {
	fixture := newCommandFixture(t, commandtest.NewScriptedPrompter())
	if err := fixture.run("login", "--unknown"); err == nil {
		t.Fatal("login accepted an unknown flag")
	}
	if err := fixture.run("login"); !errors.Is(err, commandtest.ErrScriptExhausted) {
		t.Fatalf("email prompt err = %v", err)
	}
	if err := fixture.run("login", "--email", authtest.Email); !errors.Is(err, commandtest.ErrScriptExhausted) {
		t.Fatalf("password prompt err = %v", err)
	}
	fixture.environment.Prompter = commandtest.NewScriptedPrompter("wrong")
	if err := fixture.run("login", "--email", authtest.Email); !platform.IsAPIStatus(err, http.StatusUnauthorized) {
		t.Fatalf("wrong password err = %v", err)
	}
}

func TestLogoutEndsTheSession(t *testing.T) {
	fixture := newCommandFixture(t, nil)
	fixture.login(t)

	if err := fixture.run("logout"); err != nil {
		t.Fatalf("logout: %v", err)
	}

	if !fixture.core.LoggedOut || !strings.Contains(fixture.stdout.String(), "Signed out of") {
		t.Fatalf("logged out = %v, stdout = %q", fixture.core.LoggedOut, fixture.stdout)
	}
	if err := fixture.run("status"); !errors.Is(err, auth.ErrNotSignedIn) {
		t.Fatalf("status after logout err = %v", err)
	}
}

func TestCommandsFailWithoutTheInstanceURLOrALockDirectory(t *testing.T) {
	for _, name := range []string{"login", "logout", "status", "serve"} {
		fixture := newCommandFixture(t, nil)
		fixture.environment.Getenv = func(string) string { return "" }
		if err := fixture.run(name); !errors.Is(err, platform.ErrMissingInstanceURL) {
			t.Fatalf("%s without GENWORK_URL: err = %v", name, err)
		}
		fixture = newCommandFixture(t, nil)
		fixture.environment.LockDirectory = ""
		if err := fixture.run(name); err == nil {
			t.Fatalf("%s ran without a lock directory", name)
		}
	}
}

func TestServeRefusesToStartWhenNotSignedIn(t *testing.T) {
	fixture := newCommandFixture(t, nil)

	if err := fixture.run("serve"); !errors.Is(err, auth.ErrNotSignedIn) {
		t.Fatalf("err = %v, want ErrNotSignedIn", err)
	}
}

func TestServeRefusesToStartWhenTheInstanceCannotListTheTools(t *testing.T) {
	fixture := newCommandFixture(t, nil)
	fixture.login(t)

	if err := fixture.run("serve"); !platform.IsAPIStatus(err, http.StatusNotFound) {
		t.Fatalf("err = %v, want the failing capabilities call", err)
	}
}

func TestServeExposesTheToolsOfTheEnabledModulesUntilCanceled(t *testing.T) {
	fixture := newCommandFixture(t, nil)
	fixture.login(t)
	fixture.api.RespondWithFixture(t, http.MethodGet, "/api/core/v1/capabilities", http.StatusOK, "capabilities_with_gentasks.json")
	fixture.api.RespondWithFixture(t, http.MethodGet, "/api/core/v1/me", http.StatusOK, "me_reader.json")
	fixture.api.RespondWithFixture(t, http.MethodGet, "/api/gentasks/v1/boards", http.StatusOK, "boards.json")
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	fixture.environment.ServerTransport = serverTransport
	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- command.RunServe(ctx, fixture.environment) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	session, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	listed, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "gentasks_list_boards", Arguments: map[string]any{}})
	if err != nil || result.IsError {
		t.Fatalf("call: result = %+v, err = %v", result, err)
	}
	cancel()

	var names []string
	for _, tool := range listed.Tools {
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	want := []string{"core_get_capabilities", "core_get_me", "core_list_sessions", "core_revoke_session",
		"gentasks_get_board", "gentasks_get_task", "gentasks_list_boards", "gentasks_list_labels",
		"gentasks_list_my_assigned_tasks", "gentasks_list_tasks"}
	if !slices.Equal(names, want) {
		t.Fatalf("tools = %v, want the open core tools and the gentasks read tools", names)
	}
	if session.InitializeResult().ServerInfo.Version != command.Version {
		t.Fatalf("server version = %q", session.InitializeResult().ServerInfo.Version)
	}
	select {
	case err := <-served:
		if err != nil {
			t.Fatalf("serve: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serve did not stop after cancel")
	}
}

func TestToolsetsCoverCoreAndEveryModule(t *testing.T) {
	var modules []string
	for _, set := range command.Toolsets(nil) {
		modules = append(modules, set.ModuleID)
	}
	if !slices.Equal(modules, []string{"", "gentasks"}) {
		t.Fatalf("modules = %v", modules)
	}
}

func TestServeHTTPFailsFastWithoutItsConfiguration(t *testing.T) {
	fixture := newCommandFixture(t, nil)
	fixture.environment.Getenv = func(string) string { return "" }

	err := fixture.run("serve-http")

	if err == nil || !strings.Contains(err.Error(), "MCP_HTTP_ADDR") {
		t.Errorf("error = %v", err)
	}
}

func TestServeHTTPServesUntilStopped(t *testing.T) {
	fixture := newCommandFixture(t, nil)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	values := map[string]string{
		"MCP_HTTP_ADDR": address, "GENWORK_PUBLIC_URL": "https://genwork.example.com", "CORE_INTERNAL_URL": fixture.api.URL(),
		"GATEWAY_INTERNAL_URL": fixture.api.URL(),
	}
	fixture.environment.Getenv = func(name string) string { return values[name] }
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan error, 1)
	go func() { stopped <- command.Run(ctx, []string{"serve-http"}, fixture.environment) }()

	status := 0
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if response, err := http.Get("http://" + address + "/.well-known/oauth-protected-resource"); err == nil {
			status = response.StatusCode
			response.Body.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()

	if status != http.StatusOK {
		t.Errorf("metadata status = %d", status)
	}
	select {
	case err := <-stopped:
		if err != nil {
			t.Errorf("serve-http = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve-http did not stop")
	}
}
