package app_test

import (
	"net/http"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/access"
)

func (app *testApp) createLabel(boardToken, name, color string) labelBody {
	app.t.Helper()
	resp := app.do(request{method: "POST", path: "/api/gentasks/v1/boards/" + boardToken + "/labels", body: map[string]any{"name": name, "color": color}})
	expectStatus(app.t, resp, http.StatusCreated)
	var label labelBody
	resp.decode(app.t, &label)
	return label
}

func TestCreateLabelWithPaletteColor(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Board")

	label := app.createLabel(board.Token, " bug ", "red")

	if len(label.Token) != 22 || label.Name != "bug" || label.Color != "red" {
		t.Errorf("label = %+v", label)
	}
}

func TestCreateLabelValidatesNameAndColor(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Board")
	path := "/api/gentasks/v1/boards/" + board.Token + "/labels"

	expectValidationError(t, app.do(request{method: "POST", path: path, body: map[string]any{"name": "bug", "color": "#ff0000"}}), "color")
	expectValidationError(t, app.do(request{method: "POST", path: path, body: map[string]any{"name": "", "color": "red"}}), "name")
	expectError(t, app.do(request{method: "POST", path: "/api/gentasks/v1/boards/unknown/labels", body: map[string]any{"name": "bug", "color": "red"}}), http.StatusNotFound, "board_not_found")
}

func TestLabelNamesAreUniqueIgnoringCaseUntilDeleted(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Board")
	bug := app.createLabel(board.Token, "Bug", "red")
	path := "/api/gentasks/v1/boards/" + board.Token + "/labels"

	expectError(t, app.do(request{method: "POST", path: path, body: map[string]any{"name": "bug", "color": "blue"}}), http.StatusConflict, "label_name_taken")

	expectStatus(t, app.do(request{method: "DELETE", path: "/api/gentasks/v1/labels/" + bug.Token}), http.StatusNoContent)
	app.createLabel(board.Token, "bug", "blue")
	expectError(t, app.do(request{method: "DELETE", path: "/api/gentasks/v1/labels/" + bug.Token}), http.StatusNotFound, "label_not_found")
}

func TestListLabelsOrderedByName(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Board")
	app.createLabel(board.Token, "feature", "green")
	app.createLabel(board.Token, "Bug", "red")

	resp := app.do(request{method: "GET", path: "/api/gentasks/v1/boards/" + board.Token + "/labels"})

	expectStatus(t, resp, http.StatusOK)
	var body struct {
		Labels []labelBody `json:"labels"`
	}
	resp.decode(t, &body)
	if len(body.Labels) != 2 || body.Labels[0].Name != "Bug" || body.Labels[1].Name != "feature" {
		t.Errorf("labels = %+v", body.Labels)
	}
}

func TestPatchLabelChangesOnlySentFields(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Board")
	label := app.createLabel(board.Token, "Bug", "red")
	app.createLabel(board.Token, "Feature", "green")
	path := "/api/gentasks/v1/labels/" + label.Token

	resp := app.do(request{method: "PATCH", path: path, body: map[string]any{"color": "blue"}})
	expectStatus(t, resp, http.StatusOK)
	var recolored labelBody
	resp.decode(t, &recolored)
	if recolored.Name != "Bug" || recolored.Color != "blue" {
		t.Errorf("recolored = %+v", recolored)
	}

	var unchanged labelBody
	app.do(request{method: "PATCH", path: path, body: map[string]any{}}).decode(t, &unchanged)
	if unchanged != recolored {
		t.Errorf("empty patch changed the label: %+v", unchanged)
	}
	expectValidationError(t, app.do(request{method: "PATCH", path: path, body: map[string]any{"name": nil}}), "name")
	expectValidationError(t, app.do(request{method: "PATCH", path: path, body: map[string]any{"color": "black"}}), "color")
	expectError(t, app.do(request{method: "PATCH", path: path, body: map[string]any{"name": "feature"}}), http.StatusConflict, "label_name_taken")
}

func TestDeletingALabelDetachesItFromTasks(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Board")
	bug := app.createLabel(board.Token, "Bug", "red")
	feature := app.createLabel(board.Token, "Feature", "green")
	task := app.createTask(board.Token, map[string]any{"title": "Labelled", "labelTokens": []string{bug.Token, feature.Token}})

	expectStatus(t, app.do(request{method: "DELETE", path: "/api/gentasks/v1/labels/" + bug.Token}), http.StatusNoContent)

	reloaded := app.findTask(task.Token)
	if len(reloaded.Labels) != 1 || reloaded.Labels[0].Token != feature.Token {
		t.Errorf("labels = %+v", reloaded.Labels)
	}
}

func TestLabelRoutesRequirePermissions(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Board")
	label := app.createLabel(board.Token, "Bug", "red")
	reader := app.tokenWith(access.PermissionBoardRead)
	routes := []struct {
		method, path string
		needsManage  bool
	}{
		{"GET", "/api/gentasks/v1/boards/" + board.Token + "/labels", false},
		{"POST", "/api/gentasks/v1/boards/" + board.Token + "/labels", true},
		{"PATCH", "/api/gentasks/v1/labels/" + label.Token, true},
		{"DELETE", "/api/gentasks/v1/labels/" + label.Token, true},
	}
	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			expectError(t, app.do(request{method: route.method, path: route.path, anonymous: true}), http.StatusUnauthorized, "invalid_token")
			expectError(t, app.do(request{method: route.method, path: route.path, accessToken: app.tokenWith()}), http.StatusForbidden, "forbidden")
			if route.needsManage {
				expectError(t, app.do(request{method: route.method, path: route.path, accessToken: reader}), http.StatusForbidden, "forbidden")
			}
		})
	}
}
