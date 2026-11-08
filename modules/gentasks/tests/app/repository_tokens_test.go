package app_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/access"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsync"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsynctest"
)

const repositoryTokensPath = "/api/gentasks/v1/github/repository-tokens"

// connectAppForTokens stores an App with a real key, points a board at acme/api, and installs the App there.
func (app *testApp) connectAppForTokens(installations map[string]int64) {
	app.t.Helper()
	key, encoded := githubsynctest.GenerateAppPrivateKey(app.t)
	stored := githubsynctest.StoreGithubAppWithPrivateKey(app.t, app.db, githubsynctest.EncryptionKey, encoded)
	app.github.UseApp(&key.PublicKey, stored.GithubAppID, installations)
	board := app.createBoard("Platform")
	app.patchBoardGithubSettings(board.Token, map[string]any{"repository": "acme/api"})
}

// requestRepositoryToken asks as a bot member holding the permission, as a bot run does.
func (app *testApp) requestRepositoryToken(repositories ...string) response {
	app.t.Helper()
	app.addBotMember()
	bot := app.core.MintAccessToken(app.t, botToken, access.PermissionRepositoryWrite, "core.user.read")
	return app.do(request{method: "POST", path: repositoryTokensPath, body: map[string]any{"repositories": repositories}, accessToken: bot})
}

func TestABoardRepositoryGetsAScopedInstallationToken(t *testing.T) {
	app := newTestApp(t)
	app.connectAppForTokens(map[string]int64{"acme/api": 77})

	resp := app.requestRepositoryToken("Acme/API")

	expectStatus(t, resp, http.StatusOK)
	var body struct {
		Token     string `json:"token"`
		ExpiresAt string `json:"expiresAt"`
	}
	resp.decode(t, &body)
	if body.Token != githubsynctest.IssuedInstallationToken || body.ExpiresAt == "" || resp.headers.Get("Cache-Control") != "no-store" {
		t.Errorf("response = %+v, headers = %v", body, resp.headers)
	}
	requests := app.github.TokenRequests()
	if len(requests) != 1 || requests[0].InstallationID != 77 || len(requests[0].Repositories) != 1 || requests[0].Repositories[0] != "API" {
		t.Fatalf("token requests = %+v", requests)
	}
	for name, level := range githubsync.RepositoryTokenPermissions {
		if requests[0].Permissions[name] != level {
			t.Errorf("permissions = %v", requests[0].Permissions)
		}
	}
}

func TestTheTokenNeverOutlivesTheResponse(t *testing.T) {
	app := newTestApp(t)
	app.connectAppForTokens(map[string]int64{"acme/api": 77})

	expectStatus(t, app.requestRepositoryToken("acme/api"), http.StatusOK)

	var found int
	query := `SELECT count(*) FROM github_apps WHERE encode(private_key_ciphertext, 'escape') LIKE '%' || $1 || '%'`
	if err := app.db.QueryRow(context.Background(), query, githubsynctest.IssuedInstallationToken).Scan(&found); err != nil || found != 0 {
		t.Errorf("found = %d, err = %v", found, err)
	}
}

func TestARepositoryLinkedToATaskPullRequestIsKnown(t *testing.T) {
	app := newTestApp(t)
	app.connectAppForTokens(map[string]int64{"acme/api": 77, "acme/web": 77})
	board := app.createBoardWithPrefix("Web", "WEB")
	task := app.createTaskIn(board.Token, board.Columns[1].Token, "Polish")
	ctx := context.Background()
	var pullRequestID int64
	err := app.db.QueryRow(ctx, `INSERT INTO github_pull_requests (token, repository, number, created_at, updated_at)
		VALUES ('web-pull-request', 'acme/web', 9, now(), now()) RETURNING id`).Scan(&pullRequestID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(ctx, `INSERT INTO task_pull_requests (token, task_id, pull_request_id, created_at, updated_at)
		SELECT 'web-link', id, $1, now(), now() FROM tasks WHERE token = $2`, pullRequestID, task.Token); err != nil {
		t.Fatal(err)
	}

	resp := app.requestRepositoryToken("acme/api", "acme/web")

	expectStatus(t, resp, http.StatusOK)
	if requests := app.github.TokenRequests(); len(requests) != 1 || len(requests[0].Repositories) != 2 {
		t.Errorf("token requests = %+v", requests)
	}
}

func TestRepositoriesNoBoardOrTaskUsesAreRefused(t *testing.T) {
	app := newTestApp(t)
	app.connectAppForTokens(map[string]int64{"acme/api": 77, "acme/secret": 77})

	expectValidationError(t, app.requestRepositoryToken("acme/secret"), "repositories")
	expectValidationError(t, app.requestRepositoryToken("not a repository"), "repositories")
	expectValidationError(t, app.requestRepositoryToken(), "repositories")
	tooMany := make([]string, githubsync.MaxTokenRepositories+1)
	for index := range tooMany {
		tooMany[index] = "acme/repo" + string(rune('a'+index))
	}
	expectValidationError(t, app.requestRepositoryToken(tooMany...), "repositories")
	if len(app.github.TokenRequests()) != 0 {
		t.Error("a token was requested for a refused repository")
	}
}

func TestARepositoryWithoutTheAppInstalledIsReported(t *testing.T) {
	app := newTestApp(t)
	app.connectAppForTokens(map[string]int64{})

	expectError(t, app.requestRepositoryToken("acme/api"), http.StatusConflict, "github_app_not_installed")
}

func TestAnAppWithoutTheNewPermissionsIsReported(t *testing.T) {
	app := newTestApp(t)
	app.connectAppForTokens(map[string]int64{"acme/api": 77})
	app.github.RefuseTokenPermissions()

	expectError(t, app.requestRepositoryToken("acme/api"), http.StatusConflict, "github_app_permissions_missing")
}

func TestRepositoriesOfTwoInstallationsCannotShareAToken(t *testing.T) {
	app := newTestApp(t)
	app.connectAppForTokens(map[string]int64{"acme/api": 77, "other/api": 88})
	board := app.createBoard("Other")
	app.patchBoardGithubSettings(board.Token, map[string]any{"repository": "other/api"})

	expectError(t, app.requestRepositoryToken("acme/api", "other/api"), http.StatusConflict, "github_repositories_across_installations")
}

func TestTokensNeedAConnectedAppWithAReadableKey(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Platform")
	app.patchBoardGithubSettings(board.Token, map[string]any{"repository": "acme/api"})

	expectError(t, app.requestRepositoryToken("acme/api"), http.StatusNotFound, "github_not_connected")
	githubsynctest.StoreGithubApp(t, app.db, githubsynctest.EncryptionKey)
	expectError(t, app.requestRepositoryToken("acme/api"), http.StatusConflict, "github_app_credentials_unreadable")
}

func TestTokensNeedTheRepositoryWritePermission(t *testing.T) {
	app := newTestApp(t)
	app.connectAppForTokens(map[string]int64{"acme/api": 77})
	app.addBotMember()
	bot := app.core.MintAccessToken(t, botToken, access.PermissionBoardRead, access.PermissionTaskWrite)

	resp := app.do(request{method: "POST", path: repositoryTokensPath, body: map[string]any{"repositories": []string{"acme/api"}}, accessToken: bot})

	expectStatus(t, resp, http.StatusForbidden)
}

func TestPeopleNeverGetRepositoryTokensEvenAsAdmins(t *testing.T) {
	app := newTestApp(t)
	app.connectAppForTokens(map[string]int64{"acme/api": 77})

	resp := app.do(request{method: "POST", path: repositoryTokensPath, body: map[string]any{"repositories": []string{"acme/api"}}})

	expectError(t, resp, http.StatusForbidden, "repository_tokens_for_bots_only")
	if len(app.github.TokenRequests()) != 0 {
		t.Error("a token was requested for a person")
	}
}

func TestCoreBeingDownRefusesTokens(t *testing.T) {
	app := newTestApp(t)
	app.connectAppForTokens(map[string]int64{"acme/api": 77})
	app.core.FailUserDirectory()

	expectError(t, app.requestRepositoryToken("acme/api"), http.StatusServiceUnavailable, "core_unavailable")
}
