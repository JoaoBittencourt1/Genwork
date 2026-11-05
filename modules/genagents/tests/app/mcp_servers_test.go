package app_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/access"
)

type mcpServerBody struct {
	Token     string   `json:"token"`
	Name      string   `json:"name"`
	Transport string   `json:"transport"`
	URL       *string  `json:"url"`
	Command   *string  `json:"command"`
	Args      []string `json:"args"`
	Headers   []struct {
		Name     string  `json:"name"`
		Secret   bool    `json:"secret"`
		HasValue bool    `json:"hasValue"`
		Value    *string `json:"value"`
	} `json:"headers"`
	Env []struct {
		Name     string  `json:"name"`
		Secret   bool    `json:"secret"`
		HasValue bool    `json:"hasValue"`
		Value    *string `json:"value"`
	} `json:"env"`
}

func (app *testApp) createMCPServer(body map[string]any) response {
	return app.do(request{method: "POST", path: "/api/genagents/v1/mcp-servers", body: body})
}

func (app *testApp) createSentry() mcpServerBody {
	app.t.Helper()
	resp := app.createMCPServer(map[string]any{
		"name": "sentry", "transport": "http", "url": "https://mcp.sentry.dev/mcp",
		"headers": []map[string]any{{"name": "Authorization", "secret": true, "value": "Bearer top-secret"}, {"name": "X-Team", "value": "core"}},
	})
	expectStatus(app.t, resp, http.StatusCreated)
	var server mcpServerBody
	resp.decode(app.t, &server)
	return server
}

func TestHTTPServerNeverReturnsSecretHeaderValues(t *testing.T) {
	app := newTestApp(t)

	server := app.createSentry()

	if server.URL == nil || *server.URL != "https://mcp.sentry.dev/mcp" || len(server.Headers) != 2 {
		t.Fatalf("server = %+v", server)
	}
	secret, plain := server.Headers[0], server.Headers[1]
	if !secret.Secret || !secret.HasValue || secret.Value != nil || plain.Secret || plain.Value == nil || *plain.Value != "core" {
		t.Errorf("headers = %+v", server.Headers)
	}
	for _, path := range []string{"/api/genagents/v1/mcp-servers"} {
		if resp := app.do(request{method: "GET", path: path}); strings.Contains(string(resp.body), "top-secret") {
			t.Errorf("%s leaks the secret: %s", path, resp.body)
		}
	}
	var config []byte
	var ciphertext []byte
	if err := app.db.QueryRow(context.Background(), `SELECT config, secrets_ciphertext FROM mcp_servers WHERE token = $1`, server.Token).Scan(&config, &ciphertext); err != nil {
		t.Fatalf("read stored server: %v", err)
	}
	if strings.Contains(string(config), "top-secret") || strings.Contains(string(ciphertext), "top-secret") || len(ciphertext) == 0 {
		t.Errorf("secret stored in clear: config %s", config)
	}
}

func TestStdioServer(t *testing.T) {
	app := newTestApp(t)

	resp := app.createMCPServer(map[string]any{
		"name": "files", "transport": "stdio", "command": "npx", "args": []string{"-y", "files-mcp"},
		"env": []map[string]any{{"name": "TOKEN", "secret": true, "value": "env-secret"}},
	})

	expectStatus(t, resp, http.StatusCreated)
	var server mcpServerBody
	resp.decode(t, &server)
	if server.Command == nil || *server.Command != "npx" || len(server.Args) != 2 || len(server.Env) != 1 || server.Env[0].Value != nil || !server.Env[0].HasValue {
		t.Errorf("server = %+v", server)
	}
	if strings.Contains(string(resp.body), "env-secret") {
		t.Errorf("response leaks the secret: %s", resp.body)
	}
}

func TestMCPServerValidation(t *testing.T) {
	app := newTestApp(t)
	cases := []struct {
		name  string
		body  map[string]any
		field string
	}{
		{"unknown transport", map[string]any{"name": "x", "transport": "ws"}, "transport"},
		{"http without url", map[string]any{"name": "x", "transport": "http"}, "url"},
		{"http bad url", map[string]any{"name": "x", "transport": "http", "url": "ftp://x"}, "url"},
		{"stdio without command", map[string]any{"name": "x", "transport": "stdio"}, "command"},
		{"blank name", map[string]any{"name": " ", "transport": "http", "url": "https://x.io"}, "name"},
		{"secret without value", map[string]any{"name": "x", "transport": "http", "url": "https://x.io", "headers": []map[string]any{{"name": "A", "secret": true}}}, "headers"},
		{"duplicate header", map[string]any{"name": "x", "transport": "http", "url": "https://x.io", "headers": []map[string]any{{"name": "A", "value": "1"}, {"name": "A", "value": "2"}}}, "headers"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			expectValidationError(t, app.createMCPServer(testCase.body), testCase.field)
		})
	}
}

func TestMCPServerNamesAreUnique(t *testing.T) {
	app := newTestApp(t)
	app.createSentry()

	resp := app.createMCPServer(map[string]any{"name": "sentry", "transport": "http", "url": "https://x.io"})

	expectError(t, resp, http.StatusConflict, "mcp_server_name_taken")
}

func TestPatchMCPServerKeepsUntouchedFieldsAndSecrets(t *testing.T) {
	app := newTestApp(t)
	server := app.createSentry()
	path := "/api/genagents/v1/mcp-servers/" + server.Token

	renamed := app.do(request{method: "PATCH", path: path, body: map[string]any{"name": "sentry-prod"}})
	expectStatus(t, renamed, http.StatusOK)
	var afterRename mcpServerBody
	renamed.decode(t, &afterRename)
	if afterRename.Name != "sentry-prod" || afterRename.URL == nil || *afterRename.URL != "https://mcp.sentry.dev/mcp" || len(afterRename.Headers) != 2 {
		t.Errorf("after rename = %+v", afterRename)
	}

	keep := app.do(request{method: "PATCH", path: path, body: map[string]any{"headers": []map[string]any{{"name": "Authorization", "secret": true}}}})
	expectStatus(t, keep, http.StatusOK)
	var plaintext string
	secrets, err := readStoredSecrets(app, server.Token)
	if err != nil {
		t.Fatalf("read secrets: %v", err)
	}
	plaintext = secrets
	if !strings.Contains(plaintext, "top-secret") || strings.Contains(plaintext, "X-Team") {
		t.Errorf("secrets after keeping = %s", plaintext)
	}

	missing := app.do(request{method: "PATCH", path: path, body: map[string]any{"headers": []map[string]any{{"name": "Other", "secret": true}}}})
	expectValidationError(t, missing, "headers")
	expectValidationError(t, app.do(request{method: "PATCH", path: path, body: map[string]any{"url": nil}}), "url")
	expectValidationError(t, app.do(request{method: "PATCH", path: path, body: map[string]any{"url": "nope"}}), "url")
	expectError(t, app.do(request{method: "PATCH", path: "/api/genagents/v1/mcp-servers/missing", body: map[string]any{"name": "x"}}), http.StatusNotFound, "mcp_server_not_found")
}

func TestDeleteMCPServerClearsItsSecrets(t *testing.T) {
	app := newTestApp(t)
	server := app.createSentry()

	expectStatus(t, app.do(request{method: "DELETE", path: "/api/genagents/v1/mcp-servers/" + server.Token}), http.StatusNoContent)

	var ciphertext []byte
	if err := app.db.QueryRow(context.Background(), `SELECT secrets_ciphertext FROM mcp_servers WHERE token = $1`, server.Token).Scan(&ciphertext); err != nil {
		t.Fatalf("read stored server: %v", err)
	}
	if ciphertext != nil {
		t.Error("a removed server kept its secrets")
	}
	expectError(t, app.do(request{method: "DELETE", path: "/api/genagents/v1/mcp-servers/" + server.Token}), http.StatusNotFound, "mcp_server_not_found")
}

func TestMCPServerManagementNeedsThePermission(t *testing.T) {
	app := newTestApp(t)
	member := app.tokenWith(access.PermissionBotRead)

	for _, call := range []request{
		{method: "GET", path: "/api/genagents/v1/mcp-servers"},
		{method: "POST", path: "/api/genagents/v1/mcp-servers", body: map[string]any{}},
		{method: "PATCH", path: "/api/genagents/v1/mcp-servers/x", body: map[string]any{}},
		{method: "DELETE", path: "/api/genagents/v1/mcp-servers/x"},
	} {
		call.accessToken = member
		expectError(t, app.do(call), http.StatusForbidden, "forbidden")
	}
}
