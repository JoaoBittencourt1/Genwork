package app_test

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/access"
)

type boardGithubSettingsBody struct {
	OpenedColumnToken          *string  `json:"openedColumnToken"`
	ReviewRequestedColumnToken *string  `json:"reviewRequestedColumnToken"`
	MergedColumnToken          *string  `json:"mergedColumnToken"`
	FinalBranches              []string `json:"finalBranches"`
}

func githubSettingsPath(boardToken string) string {
	return "/api/gentasks/v1/boards/" + boardToken + "/github-settings"
}

func (app *testApp) readBoardGithubSettings(boardToken string) boardGithubSettingsBody {
	app.t.Helper()
	resp := app.do(request{method: "GET", path: githubSettingsPath(boardToken)})
	expectStatus(app.t, resp, http.StatusOK)
	var settings boardGithubSettingsBody
	resp.decode(app.t, &settings)
	return settings
}

func (app *testApp) patchBoardGithubSettings(boardToken string, body map[string]any) boardGithubSettingsBody {
	app.t.Helper()
	resp := app.do(request{method: "PATCH", path: githubSettingsPath(boardToken), body: body})
	expectStatus(app.t, resp, http.StatusOK)
	var settings boardGithubSettingsBody
	resp.decode(app.t, &settings)
	return settings
}

func (app *testApp) addColumn(boardToken, name string) columnBody {
	app.t.Helper()
	resp := app.do(request{method: "POST", path: "/api/gentasks/v1/boards/" + boardToken + "/columns", body: map[string]any{"name": name}})
	expectStatus(app.t, resp, http.StatusCreated)
	var column columnBody
	resp.decode(app.t, &column)
	return column
}

func tokenOrNil(token *string) string {
	if token == nil {
		return "<nil>"
	}
	return *token
}

func TestBoardGithubSettingsDefaults(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Platform")

	settings := app.readBoardGithubSettings(board.Token)

	if settings.OpenedColumnToken != nil || settings.ReviewRequestedColumnToken != nil || tokenOrNil(settings.MergedColumnToken) != board.Columns[2].Token {
		t.Errorf("settings = opened %s, review %s, merged %s", tokenOrNil(settings.OpenedColumnToken), tokenOrNil(settings.ReviewRequestedColumnToken), tokenOrNil(settings.MergedColumnToken))
	}
	if !slices.Equal(settings.FinalBranches, []string{"main", "master"}) {
		t.Errorf("final branches = %v", settings.FinalBranches)
	}
}

func TestBoardGithubSettingsDefaultWithoutDoneColumn(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Platform")
	expectStatus(t, app.do(request{method: "DELETE", path: "/api/gentasks/v1/columns/" + board.Columns[2].Token}), http.StatusNoContent)

	if settings := app.readBoardGithubSettings(board.Token); settings.MergedColumnToken != nil {
		t.Errorf("merged = %s", *settings.MergedColumnToken)
	}
}

func TestBoardGithubSettingsUnknownBoard(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Platform")
	expectStatus(t, app.do(request{method: "DELETE", path: "/api/gentasks/v1/boards/" + board.Token}), http.StatusNoContent)

	for _, token := range []string{"missing-board", board.Token} {
		expectError(t, app.do(request{method: "GET", path: githubSettingsPath(token)}), http.StatusNotFound, "board_not_found")
		expectError(t, app.do(request{method: "PATCH", path: githubSettingsPath(token), body: map[string]any{}}), http.StatusNotFound, "board_not_found")
	}
}

func TestPatchBoardGithubSettingsSingleFieldKeepsTheOthers(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Platform")
	review := app.addColumn(board.Token, "In review")

	patched := app.patchBoardGithubSettings(board.Token, map[string]any{"reviewRequestedColumnToken": review.Token})

	if tokenOrNil(patched.ReviewRequestedColumnToken) != review.Token || patched.OpenedColumnToken != nil ||
		tokenOrNil(patched.MergedColumnToken) != board.Columns[2].Token || !slices.Equal(patched.FinalBranches, []string{"main", "master"}) {
		t.Errorf("patched = %+v", patched)
	}
	if read := app.readBoardGithubSettings(board.Token); tokenOrNil(read.ReviewRequestedColumnToken) != review.Token || tokenOrNil(read.MergedColumnToken) != board.Columns[2].Token {
		t.Errorf("read = %+v", read)
	}

	patched = app.patchBoardGithubSettings(board.Token, map[string]any{"finalBranches": []string{"main", "staging"}})

	if tokenOrNil(patched.ReviewRequestedColumnToken) != review.Token || tokenOrNil(patched.MergedColumnToken) != board.Columns[2].Token ||
		patched.OpenedColumnToken != nil || !slices.Equal(patched.FinalBranches, []string{"main", "staging"}) {
		t.Errorf("patched = %+v", patched)
	}
	if unchanged := app.patchBoardGithubSettings(board.Token, map[string]any{}); !slices.Equal(unchanged.FinalBranches, []string{"main", "staging"}) {
		t.Errorf("empty patch = %+v", unchanged)
	}
}

func TestPatchBoardGithubSettingsNullStopsMoving(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Platform")

	patched := app.patchBoardGithubSettings(board.Token, map[string]any{"mergedColumnToken": nil})

	if patched.MergedColumnToken != nil {
		t.Errorf("merged = %s", *patched.MergedColumnToken)
	}
	if read := app.readBoardGithubSettings(board.Token); read.MergedColumnToken != nil {
		t.Errorf("a cleared merged column must not fall back to done, got %s", *read.MergedColumnToken)
	}
}

func TestPatchBoardGithubSettingsRejectsForeignAndDeletedColumns(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Platform")
	other := app.createBoard("Ops")
	removed := app.addColumn(board.Token, "Removed")
	expectStatus(t, app.do(request{method: "DELETE", path: "/api/gentasks/v1/columns/" + removed.Token}), http.StatusNoContent)

	for _, field := range []string{"openedColumnToken", "reviewRequestedColumnToken", "mergedColumnToken"} {
		for _, token := range []string{other.Columns[0].Token, removed.Token, "missing-column"} {
			expectValidationError(t, app.do(request{method: "PATCH", path: githubSettingsPath(board.Token), body: map[string]any{field: token}}), field)
		}
	}
}

func TestBoardGithubSettingsTreatDeletedColumnAsUnset(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Platform")
	inProgress := app.addColumn(board.Token, "In progress")
	app.patchBoardGithubSettings(board.Token, map[string]any{"openedColumnToken": inProgress.Token})
	expectStatus(t, app.do(request{method: "DELETE", path: "/api/gentasks/v1/columns/" + inProgress.Token}), http.StatusNoContent)

	if settings := app.readBoardGithubSettings(board.Token); settings.OpenedColumnToken != nil {
		t.Errorf("opened = %s", *settings.OpenedColumnToken)
	}
}

func TestPatchBoardGithubSettingsValidatesFinalBranches(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Platform")
	tooMany := make([]string, 21)
	for index := range tooMany {
		tooMany[index] = fmt.Sprintf("release-%d", index)
	}

	for _, branches := range []any{nil, []string{}, []string{" "}, []string{"feature x"}, []string{"ma\tin"}, []string{strings.Repeat("b", 256)}, tooMany} {
		expectValidationError(t, app.do(request{method: "PATCH", path: githubSettingsPath(board.Token), body: map[string]any{"finalBranches": branches}}), "finalBranches")
	}

	patched := app.patchBoardGithubSettings(board.Token, map[string]any{"finalBranches": []string{"main", " staging ", "main", "Main"}})
	if !slices.Equal(patched.FinalBranches, []string{"main", "staging", "Main"}) {
		t.Errorf("final branches = %v, want trimmed, deduplicated, case-sensitive", patched.FinalBranches)
	}
}

func TestBoardGithubSettingsPermissions(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Platform")
	reader := app.tokenWith(access.PermissionBoardRead)

	expectStatus(t, app.do(request{method: "GET", path: githubSettingsPath(board.Token), accessToken: reader}), http.StatusOK)
	expectError(t, app.do(request{method: "GET", path: githubSettingsPath(board.Token), accessToken: app.tokenWith()}), http.StatusForbidden, "forbidden")
	expectError(t, app.do(request{method: "PATCH", path: githubSettingsPath(board.Token), accessToken: reader, body: map[string]any{"mergedColumnToken": nil}}), http.StatusForbidden, "forbidden")
}
