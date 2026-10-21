package app_test

import (
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/coreclient"
)

const assignedTasksPath = "/api/gentasks/v1/me/assigned-tasks"

type assignedTaskBody struct {
	taskBody
	Board struct {
		Token      string `json:"token"`
		Name       string `json:"name"`
		TaskPrefix string `json:"taskPrefix"`
	} `json:"board"`
	Column struct {
		Token  string  `json:"token"`
		Preset *string `json:"preset"`
		Name   *string `json:"name"`
	} `json:"column"`
}

func (app *testApp) listAssignedTasks(query string) []assignedTaskBody {
	app.t.Helper()
	resp := app.do(request{method: "GET", path: assignedTasksPath + query})
	expectStatus(app.t, resp, http.StatusOK)
	var listed struct {
		Tasks []assignedTaskBody `json:"tasks"`
	}
	resp.decode(app.t, &listed)
	return listed.Tasks
}

func (app *testApp) patchTask(token string, body map[string]any) taskBody {
	app.t.Helper()
	resp := app.do(request{method: "PATCH", path: "/api/gentasks/v1/tasks/" + token, body: body})
	expectStatus(app.t, resp, http.StatusOK)
	var task taskBody
	resp.decode(app.t, &task)
	return task
}

func assignedTitles(listed []assignedTaskBody) []string {
	titles := make([]string, 0, len(listed))
	for _, task := range listed {
		titles = append(titles, task.Title)
	}
	return titles
}

func TestAssignmentTimeFollowsAssigneeChanges(t *testing.T) {
	app := newTestApp(t)
	app.core.AddUser(coreclient.DirectoryUser{Token: "bruno-token", Name: "Bruno", Email: "bruno@example.com"})
	board := app.createBoard("Board")
	unassigned := app.createTask(board.Token, map[string]any{"title": "Nobody"})
	task := app.createTask(board.Token, map[string]any{"title": "Mine", "assigneeToken": userToken})
	createdAt := task.CreatedAt

	if unassigned.AssignedAt != nil || task.AssignedAt == nil || !task.AssignedAt.Equal(createdAt) {
		t.Fatalf("assignedAt = %v (unassigned %v), want %v", task.AssignedAt, unassigned.AssignedAt, createdAt)
	}
	app.clock.Advance(time.Hour)
	if kept := app.patchTask(task.Token, map[string]any{"urgency": "high"}); !kept.AssignedAt.Equal(createdAt) {
		t.Errorf("after another field = %v", kept.AssignedAt)
	}
	if kept := app.patchTask(task.Token, map[string]any{"assigneeToken": userToken}); !kept.AssignedAt.Equal(createdAt) {
		t.Errorf("after the same assignee = %v", kept.AssignedAt)
	}
	expectStatus(t, app.moveTask(task.Token, board.Columns[1].Token, 0), http.StatusNoContent)
	expectStatus(t, app.do(request{method: "POST", path: "/api/gentasks/v1/tasks/" + task.Token + "/archive"}), http.StatusNoContent)
	expectStatus(t, app.do(request{method: "POST", path: "/api/gentasks/v1/tasks/" + task.Token + "/unarchive"}), http.StatusNoContent)
	if kept := app.findTask(task.Token); !kept.AssignedAt.Equal(createdAt) {
		t.Errorf("after move, archive, and restore = %v", kept.AssignedAt)
	}
	if reassigned := app.patchTask(task.Token, map[string]any{"assigneeToken": "bruno-token"}); !reassigned.AssignedAt.Equal(app.clock.Now()) {
		t.Errorf("after reassigning = %v, want %v", reassigned.AssignedAt, app.clock.Now())
	}
	if cleared := app.patchTask(task.Token, map[string]any{"assigneeToken": nil}); cleared.AssignedAt != nil {
		t.Errorf("after unassigning = %v", cleared.AssignedAt)
	}
}

func TestAssignedTasksListOldestAssignmentFirstAcrossBoards(t *testing.T) {
	app := newTestApp(t)
	engineering := app.createBoard("Engineering")
	marketing := app.createBoard("Marketing")
	third := app.createTask(engineering.Token, map[string]any{"title": "Third"})
	app.clock.Advance(time.Hour)
	app.createTask(marketing.Token, map[string]any{"title": "First", "assigneeToken": userToken})
	app.clock.Advance(time.Hour)
	app.createTask(engineering.Token, map[string]any{"title": "Second", "assigneeToken": userToken, "columnToken": engineering.Columns[1].Token})
	app.clock.Advance(time.Hour)
	app.patchTask(third.Token, map[string]any{"assigneeToken": userToken})

	listed := app.listAssignedTasks("")

	if titles := assignedTitles(listed); !slices.Equal(titles, []string{"First", "Second", "Third"}) {
		t.Fatalf("titles = %v", titles)
	}
	second := listed[1]
	if second.Board.Token != engineering.Token || second.Board.Name != "Engineering" || second.Board.TaskPrefix != engineering.TaskPrefix ||
		second.Column.Token != engineering.Columns[1].Token || second.Column.Preset == nil || *second.Column.Preset != "todo" ||
		second.Number != 2 || second.AssignedAt == nil {
		t.Errorf("second = %+v", second)
	}
}

func TestAssignedTasksLeaveOutOtherUsersFinishedAndHiddenTasks(t *testing.T) {
	app := newTestApp(t)
	app.core.AddUser(coreclient.DirectoryUser{Token: "bruno-token", Name: "Bruno", Email: "bruno@example.com"})
	board := app.createBoard("Board")
	removed := app.createBoard("Removed")
	done := board.Columns[2].Token
	app.createTask(board.Token, map[string]any{"title": "Open", "assigneeToken": userToken})
	app.createTask(board.Token, map[string]any{"title": "Bruno's", "assigneeToken": "bruno-token"})
	app.createTask(board.Token, map[string]any{"title": "Done", "assigneeToken": userToken, "columnToken": done})
	archived := app.createTask(board.Token, map[string]any{"title": "Archived", "assigneeToken": userToken})
	deleted := app.createTask(board.Token, map[string]any{"title": "Deleted", "assigneeToken": userToken})
	app.createTask(removed.Token, map[string]any{"title": "On a removed board", "assigneeToken": userToken})
	expectStatus(t, app.do(request{method: "POST", path: "/api/gentasks/v1/tasks/" + archived.Token + "/archive"}), http.StatusNoContent)
	expectStatus(t, app.do(request{method: "DELETE", path: "/api/gentasks/v1/tasks/" + deleted.Token}), http.StatusNoContent)
	expectStatus(t, app.do(request{method: "DELETE", path: "/api/gentasks/v1/boards/" + removed.Token}), http.StatusNoContent)
	expectStatus(t, app.do(request{method: "PATCH", path: "/api/gentasks/v1/columns/" + done, body: map[string]any{"name": "Shipped"}}), http.StatusOK)

	if titles := assignedTitles(app.listAssignedTasks("")); !slices.Equal(titles, []string{"Open"}) {
		t.Errorf("titles = %v", titles)
	}
}

func TestAssignedTasksWithNothingAssigned(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Board")
	app.createTask(board.Token, map[string]any{"title": "Unassigned"})

	resp := app.do(request{method: "GET", path: assignedTasksPath})

	expectStatus(t, resp, http.StatusOK)
	if string(resp.body) != "{\"tasks\":[]}\n" {
		t.Errorf("body = %s", resp.body)
	}
}

func TestAssignedTasksLimit(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Board")
	for index := range 25 {
		app.createTask(board.Token, map[string]any{"title": fmt.Sprintf("Task %02d", index), "assigneeToken": userToken})
		app.clock.Advance(time.Minute)
	}

	if titles := assignedTitles(app.listAssignedTasks("?limit=3")); !slices.Equal(titles, []string{"Task 00", "Task 01", "Task 02"}) {
		t.Errorf("limit 3 = %v", titles)
	}
	if listed := app.listAssignedTasks(""); len(listed) != 20 {
		t.Errorf("default limit returned %d tasks", len(listed))
	}
	if listed := app.listAssignedTasks("?limit=50"); len(listed) != 25 {
		t.Errorf("limit 50 returned %d tasks", len(listed))
	}
	for _, limit := range []string{"0", "51", "abc", "-1"} {
		expectValidationError(t, app.do(request{method: "GET", path: assignedTasksPath + "?limit=" + limit}), "limit")
	}
}
