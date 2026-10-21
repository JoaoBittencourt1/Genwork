package app_test

import (
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/access"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/coreclient"
)

func (app *testApp) createTask(boardToken string, body map[string]any) taskBody {
	app.t.Helper()
	resp := app.do(request{method: "POST", path: "/api/gentasks/v1/boards/" + boardToken + "/tasks", body: body})
	expectStatus(app.t, resp, http.StatusCreated)
	var task taskBody
	resp.decode(app.t, &task)
	return task
}

func (app *testApp) findTask(token string) taskBody {
	app.t.Helper()
	resp := app.do(request{method: "GET", path: "/api/gentasks/v1/tasks/" + token})
	expectStatus(app.t, resp, http.StatusOK)
	var task taskBody
	resp.decode(app.t, &task)
	return task
}

func (app *testApp) moveTask(token, columnToken string, position int) response {
	return app.do(request{method: "POST", path: "/api/gentasks/v1/tasks/" + token + "/move", body: map[string]any{
		"columnToken": columnToken, "position": position,
	}})
}

// titlesOf lists the active task titles of a column, checking that positions are contiguous.
func (app *testApp) titlesOf(boardToken, columnToken string) []string {
	app.t.Helper()
	for _, column := range app.readBoard(boardToken).Columns {
		if column.Token != columnToken {
			continue
		}
		titles := []string{}
		for position, task := range column.Tasks {
			if task.Position == nil || *task.Position != position {
				app.t.Fatalf("task %q has position %v, want %d", task.Title, task.Position, position)
			}
			titles = append(titles, task.Title)
		}
		return titles
	}
	app.t.Fatalf("column %s not found", columnToken)
	return nil
}

func TestCreateMinimalTaskLandsLastInFirstColumn(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Board")
	app.createTask(board.Token, map[string]any{"title": "First"})

	task := app.createTask(board.Token, map[string]any{"title": "  Second  "})

	if task.Title != "Second" || task.ColumnToken != board.Columns[0].Token || *task.Position != 1 || task.Urgency != "none" ||
		task.AssigneeToken != nil || task.Description != nil || len(task.Labels) != 0 || task.CreatedBy != userToken || task.ArchivedAt != nil {
		t.Errorf("task = %+v", task)
	}
}

func TestCreateTaskWithEveryField(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Board")
	bug := app.createLabel(board.Token, "Bug", "red")

	task := app.createTask(board.Token, map[string]any{
		"title": "Full", "description": "Steps", "columnToken": board.Columns[2].Token,
		"assigneeToken": userToken, "labelTokens": []string{bug.Token, bug.Token}, "urgency": "urgent",
	})

	if *task.Description != "Steps" || task.ColumnToken != board.Columns[2].Token || *task.AssigneeToken != userToken ||
		task.Urgency != "urgent" || len(task.Labels) != 1 || task.Labels[0].Color != "red" {
		t.Errorf("task = %+v", task)
	}
}

func TestTasksAreNumberedPerBoardWithoutReuse(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Board")
	other := app.createBoard("Other")

	first := app.createTask(board.Token, map[string]any{"title": "First"})
	second := app.createTask(board.Token, map[string]any{"title": "Second"})
	elsewhere := app.createTask(other.Token, map[string]any{"title": "Elsewhere"})
	expectStatus(t, app.do(request{method: "DELETE", path: "/api/gentasks/v1/tasks/" + second.Token}), http.StatusNoContent)
	third := app.createTask(board.Token, map[string]any{"title": "Third"})

	if first.Number != 1 || second.Number != 2 || third.Number != 3 || elsewhere.Number != 1 {
		t.Errorf("numbers = %d, %d, %d, elsewhere %d", first.Number, second.Number, third.Number, elsewhere.Number)
	}
	if listed := app.readBoard(board.Token).Columns[0].Tasks; len(listed) != 2 || listed[1].Number != 3 {
		t.Errorf("listed = %+v", listed)
	}
	app.patchBoard(board.Token, map[string]any{"taskPrefix": "NEW"})
	if found := app.findTask(first.Token); found.Number != 1 {
		t.Errorf("number after a prefix change = %d", found.Number)
	}
}

func TestCreateTaskValidatesInput(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Board")
	other := app.createBoard("Other")
	foreignLabel := app.createLabel(other.Token, "Foreign", "gray")
	path := "/api/gentasks/v1/boards/" + board.Token + "/tasks"
	cases := map[string]struct {
		body  map[string]any
		field string
	}{
		"empty title":           {map[string]any{"title": "  "}, "title"},
		"long title":            {map[string]any{"title": strings.Repeat("a", 201)}, "title"},
		"long description":      {map[string]any{"title": "t", "description": strings.Repeat("a", 10001)}, "description"},
		"unknown urgency":       {map[string]any{"title": "t", "urgency": "critical"}, "urgency"},
		"column of other board": {map[string]any{"title": "t", "columnToken": other.Columns[0].Token}, "columnToken"},
		"unknown column":        {map[string]any{"title": "t", "columnToken": "nope"}, "columnToken"},
		"label of other board":  {map[string]any{"title": "t", "labelTokens": []string{foreignLabel.Token}}, "labelTokens"},
		"unknown assignee":      {map[string]any{"title": "t", "assigneeToken": "nobody"}, "assigneeToken"},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			expectValidationError(t, app.do(request{method: "POST", path: path, body: testCase.body}), testCase.field)
		})
	}
	expectError(t, app.do(request{method: "POST", path: "/api/gentasks/v1/boards/unknown/tasks", body: map[string]any{"title": "t"}}), http.StatusNotFound, "board_not_found")
}

func TestAssigneeCheckFailsClosedWhenCoreIsDown(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Board")
	app.core.FailUserDirectory()

	resp := app.do(request{method: "POST", path: "/api/gentasks/v1/boards/" + board.Token + "/tasks", body: map[string]any{"title": "t", "assigneeToken": userToken}})

	expectError(t, resp, http.StatusServiceUnavailable, "core_unavailable")
	if len(app.readBoard(board.Token).Columns[0].Tasks) != 0 {
		t.Error("the task must not be saved")
	}
}

func TestPatchTaskWithSingleFieldLeavesEverythingElse(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Board")
	bug := app.createLabel(board.Token, "Bug", "red")
	task := app.createTask(board.Token, map[string]any{
		"title": "Keep", "description": "Body", "assigneeToken": userToken, "labelTokens": []string{bug.Token}, "columnToken": board.Columns[1].Token,
	})
	app.clock.Advance(time.Minute)

	resp := app.do(request{method: "PATCH", path: "/api/gentasks/v1/tasks/" + task.Token, body: map[string]any{"urgency": "high"}})

	expectStatus(t, resp, http.StatusOK)
	var updated taskBody
	resp.decode(t, &updated)
	if updated.Urgency != "high" || !updated.UpdatedAt.Equal(app.clock.Now()) {
		t.Errorf("updated = %+v", updated)
	}
	updated.Urgency, updated.UpdatedAt = task.Urgency, task.UpdatedAt
	if !slices.EqualFunc([]taskBody{updated}, []taskBody{task}, sameTask) {
		t.Errorf("other fields changed:\n got %+v\nwant %+v", updated, task)
	}
}

func sameTask(left, right taskBody) bool {
	return left.Token == right.Token && left.Number == right.Number && left.ColumnToken == right.ColumnToken && *left.Position == *right.Position &&
		left.Title == right.Title && *left.Description == *right.Description && *left.AssigneeToken == *right.AssigneeToken &&
		left.Urgency == right.Urgency && slices.Equal(left.Labels, right.Labels) && left.CreatedBy == right.CreatedBy &&
		left.CreatedAt.Equal(right.CreatedAt) && left.UpdatedAt.Equal(right.UpdatedAt)
}

func TestPatchTaskNullRules(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Board")
	bug := app.createLabel(board.Token, "Bug", "red")
	task := app.createTask(board.Token, map[string]any{"title": "t", "description": "d", "assigneeToken": userToken, "labelTokens": []string{bug.Token}})
	path := "/api/gentasks/v1/tasks/" + task.Token

	resp := app.do(request{method: "PATCH", path: path, body: map[string]any{"description": nil, "assigneeToken": nil, "labelTokens": []string{}}})
	expectStatus(t, resp, http.StatusOK)
	var cleared taskBody
	resp.decode(t, &cleared)
	if cleared.Description != nil || cleared.AssigneeToken != nil || len(cleared.Labels) != 0 || cleared.Title != "t" {
		t.Errorf("cleared = %+v", cleared)
	}
	for _, field := range []string{"title", "urgency", "labelTokens"} {
		expectValidationError(t, app.do(request{method: "PATCH", path: path, body: map[string]any{field: nil}}), field)
	}
	expectValidationError(t, app.do(request{method: "PATCH", path: path, body: map[string]any{"assigneeToken": "nobody"}}), "assigneeToken")
	expectValidationError(t, app.do(request{method: "PATCH", path: path, body: map[string]any{"description": strings.Repeat("a", 10001)}}), "description")
}

func TestPatchTaskReplacesLabelsAndAssignee(t *testing.T) {
	app := newTestApp(t)
	app.core.AddUser(coreclient.DirectoryUser{Token: "bruno-token", Name: "Bruno", Email: "bruno@example.com"})
	board := app.createBoard("Board")
	bug := app.createLabel(board.Token, "Bug", "red")
	feature := app.createLabel(board.Token, "Feature", "green")
	task := app.createTask(board.Token, map[string]any{"title": "t", "labelTokens": []string{bug.Token}})

	resp := app.do(request{method: "PATCH", path: "/api/gentasks/v1/tasks/" + task.Token, body: map[string]any{
		"labelTokens": []string{feature.Token}, "assigneeToken": "bruno-token", "title": "Renamed",
	}})

	expectStatus(t, resp, http.StatusOK)
	var updated taskBody
	resp.decode(t, &updated)
	if len(updated.Labels) != 1 || updated.Labels[0].Token != feature.Token || *updated.AssigneeToken != "bruno-token" || updated.Title != "Renamed" {
		t.Errorf("updated = %+v", updated)
	}
}

func TestMoveTaskAcrossColumns(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Board")
	todo, done := board.Columns[1].Token, board.Columns[2].Token
	moving := app.createTask(board.Token, map[string]any{"title": "A", "columnToken": todo})
	app.createTask(board.Token, map[string]any{"title": "B", "columnToken": todo})
	app.createTask(board.Token, map[string]any{"title": "Done 1", "columnToken": done})

	expectStatus(t, app.moveTask(moving.Token, done, 0), http.StatusNoContent)

	if titles := app.titlesOf(board.Token, todo); !slices.Equal(titles, []string{"B"}) {
		t.Errorf("todo = %v", titles)
	}
	if titles := app.titlesOf(board.Token, done); !slices.Equal(titles, []string{"A", "Done 1"}) {
		t.Errorf("done = %v", titles)
	}
	expectStatus(t, app.moveTask(moving.Token, todo, 99), http.StatusNoContent)
	if titles := app.titlesOf(board.Token, todo); !slices.Equal(titles, []string{"B", "A"}) {
		t.Errorf("todo after moving past the end = %v", titles)
	}
}

func TestReorderTaskWithinColumn(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Board")
	column := board.Columns[0].Token
	app.createTask(board.Token, map[string]any{"title": "A"})
	app.createTask(board.Token, map[string]any{"title": "B"})
	third := app.createTask(board.Token, map[string]any{"title": "C"})

	expectStatus(t, app.moveTask(third.Token, column, 0), http.StatusNoContent)
	if titles := app.titlesOf(board.Token, column); !slices.Equal(titles, []string{"C", "A", "B"}) {
		t.Fatalf("after moving up = %v", titles)
	}
	expectStatus(t, app.moveTask(third.Token, column, 2), http.StatusNoContent)
	if titles := app.titlesOf(board.Token, column); !slices.Equal(titles, []string{"A", "B", "C"}) {
		t.Errorf("after moving down = %v", titles)
	}
}

func TestMoveTaskValidation(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Board")
	other := app.createBoard("Other")
	task := app.createTask(board.Token, map[string]any{"title": "A"})

	expectValidationError(t, app.moveTask(task.Token, other.Columns[0].Token, 0), "columnToken")
	expectValidationError(t, app.moveTask(task.Token, "unknown", 0), "columnToken")
	expectValidationError(t, app.moveTask(task.Token, board.Columns[0].Token, -1), "position")
	expectValidationError(t, app.do(request{method: "POST", path: "/api/gentasks/v1/tasks/" + task.Token + "/move", body: map[string]any{"position": 0}}), "columnToken")

	expectStatus(t, app.do(request{method: "POST", path: "/api/gentasks/v1/tasks/" + task.Token + "/archive"}), http.StatusNoContent)
	expectError(t, app.moveTask(task.Token, board.Columns[1].Token, 0), http.StatusConflict, "task_archived")
}

func TestConcurrentMovesKeepPositionsContiguous(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Board")
	backlog, todo := board.Columns[0].Token, board.Columns[1].Token
	var created []taskBody
	for _, title := range []string{"A", "B", "C", "D", "E", "F"} {
		created = append(created, app.createTask(board.Token, map[string]any{"title": title}))
	}
	var wait sync.WaitGroup

	for index, task := range created {
		wait.Go(func() {
			target := []string{backlog, todo}[index%2]
			app.moveTask(task.Token, target, 0)
		})
	}
	wait.Wait()

	if total := len(app.titlesOf(board.Token, backlog)) + len(app.titlesOf(board.Token, todo)); total != 6 {
		t.Errorf("tasks on the board = %d", total)
	}
}

func TestArchiveAndRestoreTask(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Board")
	column := board.Columns[0].Token
	first := app.createTask(board.Token, map[string]any{"title": "A"})
	app.createTask(board.Token, map[string]any{"title": "B"})
	second := app.createTask(board.Token, map[string]any{"title": "C"})
	archivePath := func(token string) string { return "/api/gentasks/v1/tasks/" + token + "/archive" }

	expectStatus(t, app.do(request{method: "POST", path: archivePath(first.Token)}), http.StatusNoContent)
	app.clock.Advance(time.Minute)
	expectStatus(t, app.do(request{method: "POST", path: archivePath(second.Token)}), http.StatusNoContent)
	expectStatus(t, app.do(request{method: "POST", path: archivePath(second.Token)}), http.StatusNoContent)

	if titles := app.titlesOf(board.Token, column); !slices.Equal(titles, []string{"B"}) {
		t.Fatalf("active = %v", titles)
	}
	resp := app.do(request{method: "GET", path: "/api/gentasks/v1/boards/" + board.Token + "/tasks?archived=true"})
	expectStatus(t, resp, http.StatusOK)
	var archived struct {
		Tasks []taskBody `json:"tasks"`
	}
	resp.decode(t, &archived)
	if len(archived.Tasks) != 2 || archived.Tasks[0].Title != "C" || archived.Tasks[0].Position != nil || archived.Tasks[0].ArchivedAt == nil {
		t.Fatalf("archived = %+v", archived.Tasks)
	}

	expectStatus(t, app.do(request{method: "POST", path: "/api/gentasks/v1/tasks/" + first.Token + "/unarchive"}), http.StatusNoContent)
	expectStatus(t, app.do(request{method: "POST", path: "/api/gentasks/v1/tasks/" + first.Token + "/unarchive"}), http.StatusNoContent)
	if titles := app.titlesOf(board.Token, column); !slices.Equal(titles, []string{"B", "A"}) {
		t.Errorf("after restore = %v", titles)
	}
	var active struct {
		Tasks []taskBody `json:"tasks"`
	}
	app.do(request{method: "GET", path: "/api/gentasks/v1/boards/" + board.Token + "/tasks"}).decode(t, &active)
	if len(active.Tasks) != 2 {
		t.Errorf("active list = %+v", active.Tasks)
	}
}

func TestDeleteActiveAndArchivedTasks(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Board")
	column := board.Columns[0].Token
	active := app.createTask(board.Token, map[string]any{"title": "A"})
	app.createTask(board.Token, map[string]any{"title": "B"})
	archived := app.createTask(board.Token, map[string]any{"title": "C"})
	expectStatus(t, app.do(request{method: "POST", path: "/api/gentasks/v1/tasks/" + archived.Token + "/archive"}), http.StatusNoContent)

	expectStatus(t, app.do(request{method: "DELETE", path: "/api/gentasks/v1/tasks/" + active.Token}), http.StatusNoContent)
	expectStatus(t, app.do(request{method: "DELETE", path: "/api/gentasks/v1/tasks/" + archived.Token}), http.StatusNoContent)

	if titles := app.titlesOf(board.Token, column); !slices.Equal(titles, []string{"B"}) {
		t.Errorf("active = %v", titles)
	}
	var archivedList struct {
		Tasks []taskBody `json:"tasks"`
	}
	app.do(request{method: "GET", path: "/api/gentasks/v1/boards/" + board.Token + "/tasks?archived=true"}).decode(t, &archivedList)
	if len(archivedList.Tasks) != 0 {
		t.Errorf("archived = %+v", archivedList.Tasks)
	}
	for _, token := range []string{active.Token, archived.Token} {
		expectError(t, app.do(request{method: "GET", path: "/api/gentasks/v1/tasks/" + token}), http.StatusNotFound, "task_not_found")
		expectError(t, app.do(request{method: "DELETE", path: "/api/gentasks/v1/tasks/" + token}), http.StatusNotFound, "task_not_found")
	}
	expectError(t, app.do(request{method: "GET", path: "/api/gentasks/v1/boards/unknown/tasks"}), http.StatusNotFound, "board_not_found")
}

func TestTaskRoutesRequirePermissions(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Board")
	task := app.createTask(board.Token, map[string]any{"title": "A"})
	writer := app.tokenWith(access.PermissionBoardRead, access.PermissionTaskWrite)
	reader := app.tokenWith(access.PermissionBoardRead)
	routes := []struct {
		method, path string
		denied       string
	}{
		{"GET", "/api/gentasks/v1/boards/" + board.Token + "/tasks", ""},
		{"GET", "/api/gentasks/v1/tasks/" + task.Token, ""},
		{"POST", "/api/gentasks/v1/boards/" + board.Token + "/tasks", reader},
		{"PATCH", "/api/gentasks/v1/tasks/" + task.Token, reader},
		{"POST", "/api/gentasks/v1/tasks/" + task.Token + "/move", reader},
		{"POST", "/api/gentasks/v1/tasks/" + task.Token + "/archive", reader},
		{"POST", "/api/gentasks/v1/tasks/" + task.Token + "/unarchive", reader},
		{"DELETE", "/api/gentasks/v1/tasks/" + task.Token, writer},
	}
	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			expectError(t, app.do(request{method: route.method, path: route.path, anonymous: true}), http.StatusUnauthorized, "invalid_token")
			expectError(t, app.do(request{method: route.method, path: route.path, accessToken: app.tokenWith()}), http.StatusForbidden, "forbidden")
			if route.denied != "" {
				expectError(t, app.do(request{method: route.method, path: route.path, accessToken: route.denied}), http.StatusForbidden, "forbidden")
			}
		})
	}
}
