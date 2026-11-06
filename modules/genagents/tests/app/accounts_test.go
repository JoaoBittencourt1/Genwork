package app_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/access"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runners"
)

type accountBody struct {
	Token       string  `json:"token"`
	Name        string  `json:"name"`
	Provider    string  `json:"provider"`
	Mode        string  `json:"mode"`
	HasAPIKey   bool    `json:"hasApiKey"`
	APIKeyLast4 *string `json:"apiKeyLast4"`
	Status      string  `json:"status"`
	LastError   *string `json:"lastError"`
	SignIn      *string `json:"signInCommand"`
	Runner      *struct {
		Token string `json:"token"`
		Name  string `json:"name"`
	} `json:"runner"`
	Models []struct {
		Token  *string `json:"token"`
		ID     string  `json:"id"`
		Custom bool    `json:"custom"`
	} `json:"models"`
}

func (app *testApp) createAccount(body map[string]any) response {
	return app.do(request{method: "POST", path: "/api/genagents/v1/accounts", body: body})
}

func (app *testApp) createAPIKeyAccount(name, provider, apiKey string) accountBody {
	app.t.Helper()
	resp := app.createAccount(map[string]any{"name": name, "provider": provider, "mode": "api_key", "apiKey": apiKey})
	expectStatus(app.t, resp, http.StatusCreated)
	var account accountBody
	resp.decode(app.t, &account)
	return account
}

func (app *testApp) createRunnerReporting(name string, tools ...string) runners.Runner {
	app.t.Helper()
	ctx := context.Background()
	repository := runners.RunnerRepository{}
	runner, err := repository.Create(ctx, app.db, name, []byte("registration-"+name), app.clock.Now())
	if err != nil {
		app.t.Fatalf("create runner: %v", err)
	}
	reported := map[string]string{}
	for _, tool := range tools {
		reported[tool] = "1.0.0"
	}
	err = repository.RecordHeartbeat(ctx, app.db, runner.ID, runners.Heartbeat{
		ContainerEngine: "docker", SandboxImage: "genwork-sandbox:test", Tools: reported, HasGithubCredentials: true, MaxConcurrency: 2, At: app.clock.Now(),
	})
	if err != nil {
		app.t.Fatalf("heartbeat: %v", err)
	}
	return runner
}

func TestProvidersListFourProvidersWithModesAndModels(t *testing.T) {
	app := newTestApp(t)

	resp := app.do(request{method: "GET", path: "/api/genagents/v1/providers"})

	expectStatus(t, resp, http.StatusOK)
	var body struct {
		Providers []struct {
			Provider string   `json:"provider"`
			Modes    []string `json:"modes"`
			Models   []struct {
				ID string `json:"id"`
			} `json:"models"`
		} `json:"providers"`
	}
	resp.decode(t, &body)
	if len(body.Providers) != 4 {
		t.Fatalf("providers = %+v", body.Providers)
	}
	for _, provider := range body.Providers {
		if provider.Provider == "xai" && (len(provider.Modes) != 1 || provider.Modes[0] != "api_key") {
			t.Errorf("xai modes = %v", provider.Modes)
		}
		if len(provider.Models) == 0 {
			t.Errorf("%s has no models", provider.Provider)
		}
	}
}

func TestSeveralAccountsOfTheSameProviderAreListed(t *testing.T) {
	app := newTestApp(t)
	app.createAPIKeyAccount("Claude — João", "anthropic", "sk-ant-aaaa1111")
	app.createAPIKeyAccount("Claude — Team", "anthropic", "sk-ant-bbbb2222")

	resp := app.do(request{method: "GET", path: "/api/genagents/v1/accounts"})

	expectStatus(t, resp, http.StatusOK)
	var body struct {
		Accounts []accountBody `json:"accounts"`
	}
	resp.decode(t, &body)
	if len(body.Accounts) != 2 || body.Accounts[0].Name != "Claude — João" || body.Accounts[1].Name != "Claude — Team" {
		t.Fatalf("accounts = %+v", body.Accounts)
	}
}

func TestAPIKeyIsNeverReturnedOnlyItsLastFour(t *testing.T) {
	app := newTestApp(t)

	account := app.createAPIKeyAccount("Claude", "anthropic", "sk-ant-secret-key-9876")

	if !account.HasAPIKey || account.APIKeyLast4 == nil || *account.APIKeyLast4 != "9876" || account.Status != "ready" {
		t.Fatalf("account = %+v", account)
	}
	for _, path := range []string{"/api/genagents/v1/accounts"} {
		resp := app.do(request{method: "GET", path: path})
		if strings.Contains(string(resp.body), "sk-ant-secret-key") {
			t.Errorf("%s leaks the key: %s", path, resp.body)
		}
	}
	var stored []byte
	if err := app.db.QueryRow(context.Background(), `SELECT api_key_ciphertext FROM ai_accounts WHERE token = $1`, account.Token).Scan(&stored); err != nil {
		t.Fatalf("read ciphertext: %v", err)
	}
	if strings.Contains(string(stored), "sk-ant-secret-key") || len(stored) == 0 {
		t.Error("the key is not stored encrypted")
	}
}

func TestSubscriptionAccountWaitsForSignIn(t *testing.T) {
	app := newTestApp(t)
	runner := app.createRunnerReporting("build-box", "claude")

	resp := app.createAccount(map[string]any{"name": "Claude", "provider": "anthropic", "mode": "subscription", "runnerToken": runner.Token})

	expectStatus(t, resp, http.StatusCreated)
	var account accountBody
	resp.decode(t, &account)
	if account.Status != "pending_sign_in" || account.Runner == nil || account.Runner.Name != "build-box" || account.SignIn == nil ||
		!strings.Contains(*account.SignIn, account.Token) || account.HasAPIKey {
		t.Errorf("account = %+v", account)
	}
}

func TestSubscriptionAccountNeedsARunnerWithTheProviderCLI(t *testing.T) {
	app := newTestApp(t)
	runner := app.createRunnerReporting("build-box", "codex")

	missingCLI := app.createAccount(map[string]any{"name": "Claude", "provider": "anthropic", "mode": "subscription", "runnerToken": runner.Token})
	unknown := app.createAccount(map[string]any{"name": "Claude", "provider": "anthropic", "mode": "subscription", "runnerToken": "nope"})
	missingRunner := app.createAccount(map[string]any{"name": "Claude", "provider": "anthropic", "mode": "subscription"})

	expectValidationError(t, missingCLI, "runnerToken")
	expectValidationError(t, unknown, "runnerToken")
	expectValidationError(t, missingRunner, "runnerToken")
}

func TestAccountCreationValidation(t *testing.T) {
	app := newTestApp(t)
	cases := []struct {
		name  string
		body  map[string]any
		field string
	}{
		{"xai subscription", map[string]any{"name": "Grok", "provider": "xai", "mode": "subscription", "runnerToken": "x"}, "mode"},
		{"unknown provider", map[string]any{"name": "X", "provider": "foo", "mode": "api_key", "apiKey": "k"}, "provider"},
		{"unknown mode", map[string]any{"name": "X", "provider": "openai", "mode": "foo"}, "mode"},
		{"missing key", map[string]any{"name": "X", "provider": "openai", "mode": "api_key"}, "apiKey"},
		{"blank name", map[string]any{"name": " ", "provider": "openai", "mode": "api_key", "apiKey": "k"}, "name"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			expectValidationError(t, app.createAccount(testCase.body), testCase.field)
		})
	}
}

func TestAccountNamesAreUnique(t *testing.T) {
	app := newTestApp(t)
	app.createAPIKeyAccount("Claude", "anthropic", "key-1111")

	resp := app.createAccount(map[string]any{"name": "Claude", "provider": "openai", "mode": "api_key", "apiKey": "key-2222"})

	expectError(t, resp, http.StatusConflict, "account_name_taken")
}

func TestAccountManagementNeedsThePermission(t *testing.T) {
	app := newTestApp(t)
	member := app.tokenWith(access.PermissionBotRead)

	for _, call := range []request{
		{method: "GET", path: "/api/genagents/v1/providers"},
		{method: "GET", path: "/api/genagents/v1/accounts"},
		{method: "POST", path: "/api/genagents/v1/accounts", body: map[string]any{}},
		{method: "PATCH", path: "/api/genagents/v1/accounts/x", body: map[string]any{}},
		{method: "DELETE", path: "/api/genagents/v1/accounts/x"},
		{method: "POST", path: "/api/genagents/v1/accounts/x/models", body: map[string]any{}},
		{method: "DELETE", path: "/api/genagents/v1/accounts/x/models/y"},
	} {
		call.accessToken = member
		expectError(t, app.do(call), http.StatusForbidden, "forbidden")
	}
	expectError(t, app.do(request{method: "GET", path: "/api/genagents/v1/accounts", anonymous: true}), http.StatusUnauthorized, "invalid_token")
}

func TestPatchAccountSingleFieldLeavesOtherFieldsUnchanged(t *testing.T) {
	app := newTestApp(t)
	account := app.createAPIKeyAccount("Claude", "anthropic", "sk-aaaa-1111")
	path := "/api/genagents/v1/accounts/" + account.Token

	renamed := app.do(request{method: "PATCH", path: path, body: map[string]any{"name": "Claude Team"}})
	expectStatus(t, renamed, http.StatusOK)
	var afterRename accountBody
	renamed.decode(t, &afterRename)
	if afterRename.Name != "Claude Team" || afterRename.APIKeyLast4 == nil || *afterRename.APIKeyLast4 != "1111" || afterRename.Provider != "anthropic" {
		t.Errorf("after rename = %+v", afterRename)
	}

	rekeyed := app.do(request{method: "PATCH", path: path, body: map[string]any{"apiKey": "sk-bbbb-2222"}})
	expectStatus(t, rekeyed, http.StatusOK)
	var afterKey accountBody
	rekeyed.decode(t, &afterKey)
	if afterKey.Name != "Claude Team" || *afterKey.APIKeyLast4 != "2222" {
		t.Errorf("after replacing the key = %+v", afterKey)
	}
	expectValidationError(t, app.do(request{method: "PATCH", path: path, body: map[string]any{"name": nil}}), "name")
	expectValidationError(t, app.do(request{method: "PATCH", path: path, body: map[string]any{"apiKey": nil}}), "apiKey")
	expectError(t, app.do(request{method: "PATCH", path: "/api/genagents/v1/accounts/missing", body: map[string]any{"name": "X"}}), http.StatusNotFound, "account_not_found")
}

func TestPatchAPIKeyOfSubscriptionAccountIsRejected(t *testing.T) {
	app := newTestApp(t)
	runner := app.createRunnerReporting("build-box", "claude")
	var account accountBody
	app.createAccount(map[string]any{"name": "Claude", "provider": "anthropic", "mode": "subscription", "runnerToken": runner.Token}).decode(t, &account)

	resp := app.do(request{method: "PATCH", path: "/api/genagents/v1/accounts/" + account.Token, body: map[string]any{"apiKey": "k"}})

	expectValidationError(t, resp, "apiKey")
}

func TestCustomModels(t *testing.T) {
	app := newTestApp(t)
	account := app.createAPIKeyAccount("Claude", "anthropic", "sk-aaaa-1111")
	path := "/api/genagents/v1/accounts/" + account.Token + "/models"

	added := app.do(request{method: "POST", path: path, body: map[string]any{"modelId": "claude-next-9", "displayName": "Next 9"}})

	expectStatus(t, added, http.StatusCreated)
	var updated accountBody
	added.decode(t, &updated)
	var customToken string
	for _, model := range updated.Models {
		if model.Custom && model.ID == "claude-next-9" && model.Token != nil {
			customToken = *model.Token
		}
	}
	if customToken == "" {
		t.Fatalf("custom model missing: %+v", updated.Models)
	}
	expectError(t, app.do(request{method: "POST", path: path, body: map[string]any{"modelId": "claude-next-9"}}), http.StatusConflict, "model_already_defined")
	expectError(t, app.do(request{method: "POST", path: path, body: map[string]any{"modelId": "claude-opus-5-5"}}), http.StatusConflict, "model_already_defined")
	expectValidationError(t, app.do(request{method: "POST", path: path, body: map[string]any{"modelId": " "}}), "modelId")

	expectStatus(t, app.do(request{method: "DELETE", path: path + "/" + customToken}), http.StatusNoContent)
	expectError(t, app.do(request{method: "DELETE", path: path + "/" + customToken}), http.StatusNotFound, "model_not_found")
}

func TestRemoveAccount(t *testing.T) {
	app := newTestApp(t)
	account := app.createAPIKeyAccount("Claude", "anthropic", "sk-aaaa-1111")

	expectStatus(t, app.do(request{method: "DELETE", path: "/api/genagents/v1/accounts/" + account.Token}), http.StatusNoContent)

	expectError(t, app.do(request{method: "DELETE", path: "/api/genagents/v1/accounts/" + account.Token}), http.StatusNotFound, "account_not_found")
}
