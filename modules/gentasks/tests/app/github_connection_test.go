package app_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/access"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsynctest"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/coretest"
)

const githubWebhookURL = coretest.Issuer + "/api/gentasks/v1/github/webhook"

type githubConnectionBody struct {
	State      string  `json:"state"`
	WebhookURL string  `json:"webhookUrl"`
	AppSlug    *string `json:"appSlug"`
	OwnerLogin *string `json:"ownerLogin"`
	AppURL     *string `json:"appUrl"`
	InstallURL *string `json:"installUrl"`
}

type githubManifestBody struct {
	FormAction string `json:"formAction"`
	State      string `json:"state"`
	Manifest   struct {
		Name           string `json:"name"`
		URL            string `json:"url"`
		HookAttributes struct {
			URL    string `json:"url"`
			Active bool   `json:"active"`
		} `json:"hook_attributes"`
		RedirectURL        string            `json:"redirect_url"`
		SetupURL           string            `json:"setup_url"`
		SetupOnUpdate      bool              `json:"setup_on_update"`
		Public             bool              `json:"public"`
		DefaultPermissions map[string]string `json:"default_permissions"`
		DefaultEvents      []string          `json:"default_events"`
	} `json:"manifest"`
}

func (app *testApp) readGithubConnection() githubConnectionBody {
	app.t.Helper()
	resp := app.do(request{method: "GET", path: "/api/gentasks/v1/github/connection"})
	expectStatus(app.t, resp, http.StatusOK)
	var body githubConnectionBody
	resp.decode(app.t, &body)
	return body
}

func (app *testApp) startGithubManifest(body map[string]any) githubManifestBody {
	app.t.Helper()
	resp := app.do(request{method: "POST", path: "/api/gentasks/v1/github/connection/manifest", body: body})
	expectStatus(app.t, resp, http.StatusOK)
	var started githubManifestBody
	resp.decode(app.t, &started)
	return started
}

func (app *testApp) completeGithubConnection(code, state string) response {
	app.t.Helper()
	return app.do(request{method: "POST", path: "/api/gentasks/v1/github/connection", body: map[string]any{"code": code, "state": state}})
}

func TestGithubConnectionStartsDisconnected(t *testing.T) {
	app := newTestApp(t)

	connection := app.readGithubConnection()

	if connection.State != "disconnected" || connection.WebhookURL != githubWebhookURL || connection.AppSlug != nil || connection.InstallURL != nil {
		t.Errorf("connection = %+v", connection)
	}
}

func TestStartGithubManifestForPersonalAccount(t *testing.T) {
	app := newTestApp(t)

	started := app.startGithubManifest(map[string]any{})

	if started.FormAction != githubsynctest.UnroutableGithubWebURL+"/settings/apps/new?state="+url.QueryEscape(started.State) || started.State == "" {
		t.Errorf("form action = %q, state = %q", started.FormAction, started.State)
	}
	manifest := started.Manifest
	if !strings.HasPrefix(manifest.Name, "Genwork ") || len(manifest.Name) > 34 || manifest.URL != coretest.Issuer || manifest.Public {
		t.Errorf("manifest = %+v", manifest)
	}
	if manifest.HookAttributes.URL != githubWebhookURL || !manifest.HookAttributes.Active ||
		manifest.RedirectURL != coretest.Issuer+"/tasks/integrations/github/callback" ||
		manifest.SetupURL != coretest.Issuer+"/tasks/integrations/github" || !manifest.SetupOnUpdate {
		t.Errorf("manifest URLs = %+v", manifest)
	}
	if len(manifest.DefaultPermissions) != 2 || manifest.DefaultPermissions["pull_requests"] != "read" || manifest.DefaultPermissions["metadata"] != "read" ||
		len(manifest.DefaultEvents) != 1 || manifest.DefaultEvents[0] != "pull_request" {
		t.Errorf("permissions = %v, events = %v", manifest.DefaultPermissions, manifest.DefaultEvents)
	}
	if other := app.startGithubManifest(map[string]any{}); other.Manifest.Name == manifest.Name || other.State == started.State {
		t.Error("each start must suggest a fresh name and state")
	}
}

func TestStartGithubManifestForOrganization(t *testing.T) {
	app := newTestApp(t)

	started := app.startGithubManifest(map[string]any{"organization": " acme-labs "})

	if started.FormAction != githubsynctest.UnroutableGithubWebURL+"/organizations/acme-labs/settings/apps/new?state="+url.QueryEscape(started.State) {
		t.Errorf("form action = %q", started.FormAction)
	}
	if personal := app.startGithubManifest(map[string]any{"organization": nil}); !strings.Contains(personal.FormAction, "/settings/apps/new") ||
		strings.Contains(personal.FormAction, "/organizations/") {
		t.Errorf("null organization form action = %q", personal.FormAction)
	}
}

func TestStartGithubManifestValidatesOrganization(t *testing.T) {
	app := newTestApp(t)
	for _, organization := range []string{"-acme", "acme-", "ac me", "acme_labs", "acme--labs", strings.Repeat("a", 40)} {
		resp := app.do(request{method: "POST", path: "/api/gentasks/v1/github/connection/manifest", body: map[string]any{"organization": organization}})
		expectValidationError(t, resp, "organization")
	}
}

func TestCompleteGithubConnectionStoresEncryptedCredentials(t *testing.T) {
	app := newTestApp(t)
	started := app.startGithubManifest(map[string]any{"organization": "acme"})

	resp := app.completeGithubConnection(githubsynctest.OrganizationAppCode, started.State)

	expectStatus(t, resp, http.StatusCreated)
	var connection githubConnectionBody
	resp.decode(t, &connection)
	if connection.State != "connected" || *connection.AppSlug != "genwork-k3f9qz" || *connection.OwnerLogin != "acme" ||
		*connection.AppURL != githubsynctest.UnroutableGithubWebURL+"/organizations/acme/settings/apps/genwork-k3f9qz" ||
		*connection.InstallURL != githubsynctest.UnroutableGithubWebURL+"/apps/genwork-k3f9qz/installations/new" || connection.WebhookURL != githubWebhookURL {
		t.Errorf("connection = %+v", connection)
	}
	if read := app.readGithubConnection(); read.State != "connected" || *read.AppSlug != "genwork-k3f9qz" {
		t.Errorf("read = %+v", read)
	}
	expectNoSecretIn(t, string(resp.body))
	var storedRow string
	if err := app.db.QueryRow(context.Background(), `SELECT github_apps::text FROM github_apps`).Scan(&storedRow); err != nil {
		t.Fatal(err)
	}
	expectNoSecretIn(t, storedRow)
	if strings.Contains(storedRow, githubsynctest.ClientSecret) {
		t.Error("the OAuth client secret must be discarded")
	}
}

func TestCompleteGithubConnectionForPersonalAccount(t *testing.T) {
	app := newTestApp(t)
	started := app.startGithubManifest(map[string]any{})

	resp := app.completeGithubConnection(githubsynctest.UserAppCode, started.State)

	expectStatus(t, resp, http.StatusCreated)
	var connection githubConnectionBody
	resp.decode(t, &connection)
	if *connection.OwnerLogin != "ana" || *connection.AppURL != githubsynctest.UnroutableGithubWebURL+"/settings/apps/genwork-k3f9qz" {
		t.Errorf("connection = %+v", connection)
	}
}

func TestCompleteGithubConnectionRejectsInvalidStates(t *testing.T) {
	app := newTestApp(t)
	started := app.startGithubManifest(map[string]any{})
	otherUser := app.core.MintAccessToken(t, "bruno-user-token", access.Permissions...)
	tampered := withLastCharacterChanged(started.State)

	for _, state := range []string{"", "not-a-state", tampered} {
		expectValidationError(t, app.completeGithubConnection(githubsynctest.OrganizationAppCode, state), "state")
	}
	foreign := app.do(request{method: "POST", path: "/api/gentasks/v1/github/connection", accessToken: otherUser,
		body: map[string]any{"code": githubsynctest.OrganizationAppCode, "state": started.State}})
	expectValidationError(t, foreign, "state")
	app.clock.Advance(time.Hour + time.Second)
	expectValidationError(t, app.completeGithubConnection(githubsynctest.OrganizationAppCode, started.State), "state")

	if conversions := app.github.Conversions(); len(conversions) != 0 {
		t.Errorf("GitHub must not be called with an invalid state, got %v", conversions)
	}
}

func TestCompleteGithubConnectionValidatesCode(t *testing.T) {
	app := newTestApp(t)
	started := app.startGithubManifest(map[string]any{})
	for _, code := range []string{"", "with/slash", strings.Repeat("a", 101)} {
		expectValidationError(t, app.completeGithubConnection(code, started.State), "code")
	}
}

func TestCompleteGithubConnectionMapsGithubFailures(t *testing.T) {
	app := newTestApp(t)
	started := app.startGithubManifest(map[string]any{})

	expectValidationError(t, app.completeGithubConnection(githubsynctest.ExpiredCode, started.State), "code")
	expectValidationError(t, app.completeGithubConnection(githubsynctest.RejectedCode, started.State), "code")
	expectError(t, app.completeGithubConnection(githubsynctest.BrokenCode, started.State), http.StatusBadGateway, "github_unavailable")
	expectError(t, app.completeGithubConnection(githubsynctest.IncompleteCode, started.State), http.StatusBadGateway, "github_unavailable")

	if connection := app.readGithubConnection(); connection.State != "disconnected" {
		t.Errorf("nothing may be stored after a failure, got %+v", connection)
	}
}

func TestCompleteGithubConnectionWhenGithubIsUnreachable(t *testing.T) {
	app := newTestAppWithGithubAPI(t, githubsynctest.UnroutableGithubAPIURL)
	started := app.startGithubManifest(map[string]any{})

	expectError(t, app.completeGithubConnection(githubsynctest.OrganizationAppCode, started.State), http.StatusBadGateway, "github_unavailable")
}

func TestGithubConnectionConflictsWhenAlreadyConnected(t *testing.T) {
	app := newTestApp(t)
	started := app.startGithubManifest(map[string]any{})
	githubsynctest.StoreGithubApp(t, app.db, githubsynctest.EncryptionKey)

	expectError(t, app.do(request{method: "POST", path: "/api/gentasks/v1/github/connection/manifest", body: map[string]any{}}), http.StatusConflict, "github_already_connected")
	expectError(t, app.completeGithubConnection(githubsynctest.OrganizationAppCode, started.State), http.StatusConflict, "github_already_connected")

	if conversions := app.github.Conversions(); len(conversions) != 0 {
		t.Errorf("GitHub must not be called when already connected, got %v", conversions)
	}
}

func TestGithubConnectionUnreadableAfterKeyChange(t *testing.T) {
	app := newTestApp(t)
	githubsynctest.StoreGithubApp(t, app.db, githubsynctest.OtherEncryptionKey)

	connection := app.readGithubConnection()

	if connection.State != "unreadable" || *connection.AppSlug != "genwork-fixture" {
		t.Errorf("connection = %+v", connection)
	}
}

func TestDisconnectGithub(t *testing.T) {
	app := newTestApp(t)
	githubsynctest.StoreGithubApp(t, app.db, githubsynctest.EncryptionKey)

	expectStatus(t, app.do(request{method: "DELETE", path: "/api/gentasks/v1/github/connection"}), http.StatusNoContent)
	expectStatus(t, app.do(request{method: "DELETE", path: "/api/gentasks/v1/github/connection"}), http.StatusNoContent)

	if connection := app.readGithubConnection(); connection.State != "disconnected" {
		t.Errorf("connection = %+v", connection)
	}
}

func TestGithubConnectionRequiresPermissions(t *testing.T) {
	app := newTestApp(t)
	reader := app.tokenWith(access.PermissionBoardRead)
	nobody := app.tokenWith()

	expectStatus(t, app.do(request{method: "GET", path: "/api/gentasks/v1/github/connection", accessToken: reader}), http.StatusOK)
	expectError(t, app.do(request{method: "GET", path: "/api/gentasks/v1/github/connection", accessToken: nobody}), http.StatusForbidden, "forbidden")
	for _, req := range []request{
		{method: "POST", path: "/api/gentasks/v1/github/connection/manifest", body: map[string]any{}},
		{method: "POST", path: "/api/gentasks/v1/github/connection", body: map[string]any{"code": "x", "state": "y"}},
		{method: "DELETE", path: "/api/gentasks/v1/github/connection"},
	} {
		req.accessToken = reader
		expectError(t, app.do(req), http.StatusForbidden, "forbidden")
	}
}

func withLastCharacterChanged(text string) string {
	replacement := "A"
	if strings.HasSuffix(text, "A") {
		replacement = "B"
	}
	return text[:len(text)-1] + replacement
}

func expectNoSecretIn(t *testing.T, text string) {
	t.Helper()
	if strings.Contains(text, githubsynctest.WebhookSecret) || strings.Contains(text, "fixture-private-key") {
		t.Errorf("secret material leaked: %s", text)
	}
}
