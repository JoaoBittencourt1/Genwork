package app_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/access"
)

func TestCreateBoardStartsWithDefaultColumns(t *testing.T) {
	app := newTestApp(t)

	board := app.createBoard("  Platform  ")

	if len(board.Token) != 22 || board.Name != "Platform" || board.CreatedBy != userToken || !board.CreatedAt.Equal(app.clock.Now()) {
		t.Errorf("board = %+v", board.boardBody)
	}
	if labels := columnLabels(t, board.Columns); !slices.Equal(labels, []string{"backlog", "todo", "done"}) {
		t.Errorf("columns = %v", labels)
	}
	for _, column := range board.Columns {
		if column.Name != nil {
			t.Errorf("default column = %+v", column)
		}
	}
	read := app.readBoard(board.Token)
	for _, column := range read.Columns {
		if column.Tasks == nil || len(column.Tasks) != 0 {
			t.Errorf("read column = %+v", column)
		}
	}
	if read.Labels == nil || len(read.Labels) != 0 {
		t.Errorf("labels = %+v", read.Labels)
	}
}

func TestCreateBoardValidatesName(t *testing.T) {
	app := newTestApp(t)
	for _, name := range []string{"   ", strings.Repeat("a", 81)} {
		expectValidationError(t, app.do(request{method: "POST", path: "/api/gentasks/v1/boards", body: map[string]any{"name": name}}), "name")
	}
	expectError(t, app.do(request{method: "POST", path: "/api/gentasks/v1/boards", rawBody: `{"name":"x","owner":"me"}`}), http.StatusBadRequest, "malformed_body")
}

func TestListBoardsOrderedByName(t *testing.T) {
	app := newTestApp(t)
	app.createBoard("Zeta")
	app.createBoard("alpha")

	resp := app.do(request{method: "GET", path: "/api/gentasks/v1/boards"})

	expectStatus(t, resp, http.StatusOK)
	var body struct {
		Boards []boardBody `json:"boards"`
	}
	resp.decode(t, &body)
	if len(body.Boards) != 2 || body.Boards[0].Name != "alpha" || body.Boards[1].Name != "Zeta" {
		t.Errorf("boards = %+v", body.Boards)
	}
}

func TestRenameBoard(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Old")
	app.clock.Advance(time.Minute)

	resp := app.do(request{method: "PATCH", path: "/api/gentasks/v1/boards/" + board.Token, body: map[string]any{"name": "New"}})

	expectStatus(t, resp, http.StatusOK)
	var renamed boardBody
	resp.decode(t, &renamed)
	if renamed.Name != "New" || !renamed.UpdatedAt.Equal(app.clock.Now()) || renamed.CreatedBy != userToken {
		t.Errorf("renamed = %+v", renamed)
	}
	expectError(t, app.do(request{method: "PATCH", path: "/api/gentasks/v1/boards/" + board.Token, body: map[string]any{"name": nil}}), http.StatusBadRequest, "validation_failed")
	expectValidationError(t, app.do(request{method: "PATCH", path: "/api/gentasks/v1/boards/" + board.Token, body: map[string]any{"name": " "}}), "name")
}

func TestPatchBoardWithEmptyBodyChangesNothing(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Keep")
	app.clock.Advance(time.Minute)

	resp := app.do(request{method: "PATCH", path: "/api/gentasks/v1/boards/" + board.Token, body: map[string]any{}})

	expectStatus(t, resp, http.StatusOK)
	var unchanged boardBody
	resp.decode(t, &unchanged)
	if unchanged != board.boardBody {
		t.Errorf("unchanged = %+v, want %+v", unchanged, board.boardBody)
	}
}

func TestDeleteBoardHidesEverythingUnderIt(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Doomed")
	task := app.createTask(board.Token, map[string]any{"title": "Inside"})

	expectStatus(t, app.do(request{method: "DELETE", path: "/api/gentasks/v1/boards/" + board.Token}), http.StatusNoContent)

	expectError(t, app.do(request{method: "GET", path: "/api/gentasks/v1/boards/" + board.Token}), http.StatusNotFound, "board_not_found")
	expectError(t, app.do(request{method: "DELETE", path: "/api/gentasks/v1/boards/" + board.Token}), http.StatusNotFound, "board_not_found")
	expectError(t, app.do(request{method: "PATCH", path: "/api/gentasks/v1/columns/" + board.Columns[0].Token, body: map[string]any{"name": "x"}}), http.StatusNotFound, "column_not_found")
	expectError(t, app.do(request{method: "GET", path: "/api/gentasks/v1/tasks/" + task.Token}), http.StatusNotFound, "task_not_found")
	var listed struct {
		Boards []boardBody `json:"boards"`
	}
	app.do(request{method: "GET", path: "/api/gentasks/v1/boards"}).decode(t, &listed)
	if len(listed.Boards) != 0 {
		t.Errorf("boards = %+v", listed.Boards)
	}
}

func TestAddRenameAndMoveColumns(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Flow")

	resp := app.do(request{method: "POST", path: "/api/gentasks/v1/boards/" + board.Token + "/columns", body: map[string]any{"name": "Review"}})
	expectStatus(t, resp, http.StatusCreated)
	var review columnBody
	resp.decode(t, &review)
	if review.Position != 3 || review.Preset != nil || *review.Name != "Review" {
		t.Fatalf("review = %+v", review)
	}

	moved := app.do(request{method: "POST", path: "/api/gentasks/v1/columns/" + review.Token + "/move", body: map[string]any{"position": 1}})
	expectStatus(t, moved, http.StatusNoContent)
	if labels := columnLabels(t, app.readBoard(board.Token).Columns); !slices.Equal(labels, []string{"backlog", "Review", "todo", "done"}) {
		t.Fatalf("after move = %v", labels)
	}

	expectStatus(t, app.do(request{method: "POST", path: "/api/gentasks/v1/columns/" + review.Token + "/move", body: map[string]any{"position": 99}}), http.StatusNoContent)
	if labels := columnLabels(t, app.readBoard(board.Token).Columns); !slices.Equal(labels, []string{"backlog", "todo", "done", "Review"}) {
		t.Fatalf("after move past the end = %v", labels)
	}
	expectValidationError(t, app.do(request{method: "POST", path: "/api/gentasks/v1/columns/" + review.Token + "/move", body: map[string]any{"position": -1}}), "position")
	expectValidationError(t, app.do(request{method: "POST", path: "/api/gentasks/v1/boards/" + board.Token + "/columns", body: map[string]any{"name": ""}}), "name")
}

func TestRenamePresetColumnAndClearItsName(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Flow")
	todo := board.Columns[1]

	resp := app.do(request{method: "PATCH", path: "/api/gentasks/v1/columns/" + todo.Token, body: map[string]any{"name": "Next up"}})
	expectStatus(t, resp, http.StatusOK)
	var renamed columnBody
	resp.decode(t, &renamed)
	if *renamed.Name != "Next up" || *renamed.Preset != "todo" {
		t.Fatalf("renamed = %+v", renamed)
	}

	resp = app.do(request{method: "PATCH", path: "/api/gentasks/v1/columns/" + todo.Token, body: map[string]any{"name": nil}})
	expectStatus(t, resp, http.StatusOK)
	var cleared columnBody
	resp.decode(t, &cleared)
	if cleared.Name != nil || *cleared.Preset != "todo" {
		t.Errorf("cleared = %+v", cleared)
	}
	expectStatus(t, app.do(request{method: "PATCH", path: "/api/gentasks/v1/columns/" + todo.Token, body: map[string]any{}}), http.StatusOK)
}

func TestCustomColumnNameCannotBeCleared(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Flow")
	var review columnBody
	app.do(request{method: "POST", path: "/api/gentasks/v1/boards/" + board.Token + "/columns", body: map[string]any{"name": "Review"}}).decode(t, &review)

	expectValidationError(t, app.do(request{method: "PATCH", path: "/api/gentasks/v1/columns/" + review.Token, body: map[string]any{"name": nil}}), "name")
}

func TestDeleteColumnRules(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Flow")
	backlog, todo, done := board.Columns[0], board.Columns[1], board.Columns[2]
	archived := app.createTask(board.Token, map[string]any{"title": "Archived", "columnToken": todo.Token})
	expectStatus(t, app.do(request{method: "POST", path: "/api/gentasks/v1/tasks/" + archived.Token + "/archive"}), http.StatusNoContent)
	app.createTask(board.Token, map[string]any{"title": "Active", "columnToken": backlog.Token})

	expectError(t, app.do(request{method: "DELETE", path: "/api/gentasks/v1/columns/" + todo.Token}), http.StatusConflict, "column_not_empty")
	expectError(t, app.do(request{method: "DELETE", path: "/api/gentasks/v1/columns/" + backlog.Token}), http.StatusConflict, "column_not_empty")

	expectStatus(t, app.do(request{method: "DELETE", path: "/api/gentasks/v1/columns/" + done.Token}), http.StatusNoContent)
	expectError(t, app.do(request{method: "DELETE", path: "/api/gentasks/v1/columns/" + done.Token}), http.StatusNotFound, "column_not_found")

	other := app.createBoard("Single")
	for _, column := range other.Columns[1:] {
		expectStatus(t, app.do(request{method: "DELETE", path: "/api/gentasks/v1/columns/" + column.Token}), http.StatusNoContent)
	}
	expectError(t, app.do(request{method: "DELETE", path: "/api/gentasks/v1/columns/" + other.Columns[0].Token}), http.StatusConflict, "last_column")
}

func TestDeletingAColumnKeepsPositionsContiguous(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Flow")

	expectStatus(t, app.do(request{method: "DELETE", path: "/api/gentasks/v1/columns/" + board.Columns[0].Token}), http.StatusNoContent)

	if labels := columnLabels(t, app.readBoard(board.Token).Columns); !slices.Equal(labels, []string{"todo", "done"}) {
		t.Errorf("columns = %v", labels)
	}
}

func TestBoardRoutesRequireAuthenticationAndPermissions(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Guarded")
	column := board.Columns[0].Token
	reader := app.tokenWith(access.PermissionBoardRead)
	routes := []struct {
		method, path string
		needsManage  bool
	}{
		{"GET", "/api/gentasks/v1/boards", false},
		{"GET", "/api/gentasks/v1/boards/" + board.Token, false},
		{"POST", "/api/gentasks/v1/boards", true},
		{"PATCH", "/api/gentasks/v1/boards/" + board.Token, true},
		{"DELETE", "/api/gentasks/v1/boards/" + board.Token, true},
		{"POST", "/api/gentasks/v1/boards/" + board.Token + "/columns", true},
		{"PATCH", "/api/gentasks/v1/columns/" + column, true},
		{"POST", "/api/gentasks/v1/columns/" + column + "/move", true},
		{"DELETE", "/api/gentasks/v1/columns/" + column, true},
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
