package remote_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/remote"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/toolset/toolsettest"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/coretest"
)

const expectedChallenge = `Bearer resource_metadata="` + coretest.Issuer + `/.well-known/oauth-protected-resource/mcp"`

func postInitialize(t *testing.T, fixture *remoteFixture, authorization string) *http.Response {
	t.Helper()
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"curl","version":"1"}}}`
	request, err := http.NewRequest(http.MethodPost, fixture.server.URL+"/mcp", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { response.Body.Close() })
	return response
}

func TestRequestsWithoutAValidTokenPointToTheResourceMetadata(t *testing.T) {
	fixture := newRemoteFixture(t)
	cases := map[string]string{
		"missing": "",
		"invalid": "Bearer not-a-token",
		"expired": "Bearer " + fixture.core.MintAccessTokenExpiringAt(t, "usr_admin", time.Now().Add(-time.Minute)),
		"basic":   "Basic dXNlcjpwYXNz",
	}
	for name, authorization := range cases {
		t.Run(name, func(t *testing.T) {
			response := postInitialize(t, fixture, authorization)

			if response.StatusCode != http.StatusUnauthorized || response.Header.Get("WWW-Authenticate") != expectedChallenge {
				t.Errorf("status = %d, WWW-Authenticate = %q", response.StatusCode, response.Header.Get("WWW-Authenticate"))
			}
		})
	}
	if requests := fixture.api.Requests(); len(requests) != 0 {
		t.Errorf("no API call may happen without a valid token: %+v", requests)
	}
}

func TestProtectedResourceMetadataIsPublicAtBothPaths(t *testing.T) {
	fixture := newRemoteFixture(t)

	for _, path := range []string{"/.well-known/oauth-protected-resource/mcp", "/.well-known/oauth-protected-resource"} {
		response, err := http.Get(fixture.server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		var metadata map[string]any
		_ = json.NewDecoder(response.Body).Decode(&metadata)
		response.Body.Close()
		servers, _ := metadata["authorization_servers"].([]any)
		methods, _ := metadata["bearer_methods_supported"].([]any)
		scopes, _ := metadata["scopes_supported"].([]any)
		if response.StatusCode != http.StatusOK || metadata["resource"] != coretest.Issuer+"/mcp" ||
			len(servers) != 1 || servers[0] != coretest.Issuer || len(methods) != 1 || methods[0] != "header" || len(scopes) != 1 || scopes[0] != "genwork" {
			t.Errorf("%s = %d %v", path, response.StatusCode, metadata)
		}
	}
}

func TestAValidTokenOpensASessionWithTheUsersTools(t *testing.T) {
	fixture := newRemoteFixture(t)

	session := fixture.connect(t, nil)

	if session.ID() == "" {
		t.Error("the server must assign a session id")
	}
	names := toolsettest.ListToolNames(t, session)
	if !slices.Contains(names, "core_get_me") || !slices.Contains(names, "gentasks_create_board") {
		t.Errorf("tools = %v", names)
	}
}

func TestToolsFollowTheUsersPermissions(t *testing.T) {
	fixture := newRemoteFixture(t)
	fixture.api.RespondWithFixture(t, http.MethodGet, "/api/core/v1/me", http.StatusOK, "me_reader.json")

	names := toolsettest.ListToolNames(t, fixture.connect(t, nil))

	if !slices.Contains(names, "gentasks_list_boards") || slices.Contains(names, "gentasks_create_board") {
		t.Errorf("tools = %v", names)
	}
}

func TestToolsResyncAfterTheIntervalAndNotifyTheClient(t *testing.T) {
	fixture := newRemoteFixture(t)
	var notified atomic.Int32
	session := fixture.connect(t, func() { notified.Add(1) })
	fixture.api.RespondWithFixture(t, http.MethodGet, "/api/core/v1/capabilities", http.StatusOK, "capabilities_core_only.json")

	if names := toolsettest.ListToolNames(t, session); !slices.Contains(names, "gentasks_list_boards") {
		t.Fatalf("within the interval the tools stay: %v", names)
	}
	fixture.clock.Advance(syncInterval + time.Second)
	names := toolsettest.ListToolNames(t, session)

	if slices.Contains(names, "gentasks_list_boards") || !slices.Contains(names, "core_get_me") {
		t.Errorf("tools = %v", names)
	}
	deadline := time.Now().Add(2 * time.Second)
	for notified.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if notified.Load() == 0 {
		t.Error("the client must receive tools/list_changed")
	}
}

func TestASessionRefusesAnotherUsersToken(t *testing.T) {
	fixture := newRemoteFixture(t)
	session := fixture.connect(t, nil)
	requestsBefore := len(fixture.api.Requests())
	body := `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"core_get_me","arguments":{}}}`
	request, err := http.NewRequest(http.MethodPost, fixture.server.URL+"/mcp", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("Mcp-Session-Id", session.ID())
	request.Header.Set("Authorization", "Bearer "+fixture.core.MintAccessToken(t, "usr_someone_else"))

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()

	if response.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d", response.StatusCode)
	}
	if requests := fixture.api.Requests(); len(requests) != requestsBefore {
		t.Errorf("no tool may run: %+v", requests[requestsBefore:])
	}
}

func TestCallsUseTheTokenOfTheRequestThatCarriesThem(t *testing.T) {
	fixture := newRemoteFixture(t)
	session := fixture.connect(t, nil)
	refreshed := fixture.core.MintAccessToken(t, "usr_admin", "core.user.read")

	fixture.useToken(refreshed)
	toolsettest.CallTool(t, session, "core_get_me", map[string]any{})

	requests := fixture.api.Requests()
	last := requests[len(requests)-1]
	if last.Path != "/api/core/v1/me" || last.Authorization != "Bearer "+refreshed {
		t.Errorf("last API request = %+v", last)
	}
}

func TestLoadHTTPServerConfigNeedsEveryAddress(t *testing.T) {
	_, err := remote.LoadHTTPServerConfig(func(string) string { return "" })

	for _, name := range []string{"MCP_HTTP_ADDR", "GENWORK_PUBLIC_URL", "CORE_INTERNAL_URL", "GATEWAY_INTERNAL_URL"} {
		if err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("error %v does not name %s", err, name)
		}
	}
	values := map[string]string{
		"MCP_HTTP_ADDR": ":8092", "GENWORK_PUBLIC_URL": "https://genwork.example.com/", "CORE_INTERNAL_URL": "http://core:8484",
		"GATEWAY_INTERNAL_URL": "http://gateway:8487",
	}
	config, err := remote.LoadHTTPServerConfig(func(name string) string { return values[name] })
	if err != nil || config.PublicURL != "https://genwork.example.com" || config.Address != ":8092" {
		t.Errorf("config = %+v, err = %v", config, err)
	}
	values["GENWORK_PUBLIC_URL"] = "genwork.example.com"
	if _, err := remote.LoadHTTPServerConfig(func(name string) string { return values[name] }); err == nil || !strings.Contains(err.Error(), "GENWORK_PUBLIC_URL") {
		t.Errorf("error = %v", err)
	}
}
