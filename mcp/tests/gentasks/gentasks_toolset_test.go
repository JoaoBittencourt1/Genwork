package gentasks_test

import (
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/gentasks"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/toolset/toolsettest"
)

func TestGentasksBoardAndLabelToolsSendTheirRequestToTheModule(t *testing.T) {
	cases := []toolsettest.ToolRequestCase{
		{Tool: "gentasks_list_boards", Method: http.MethodGet, Path: "/api/gentasks/v1/boards"},
		{Tool: "gentasks_get_board", Arguments: map[string]any{"boardToken": "brd_1"}, Method: http.MethodGet, Path: "/api/gentasks/v1/boards/brd_1"},
		{
			Tool: "gentasks_create_board", Arguments: map[string]any{"name": "Engineering"},
			Method: http.MethodPost, Path: "/api/gentasks/v1/boards", Body: map[string]any{"name": "Engineering"},
		},
		{
			Tool: "gentasks_create_board", Arguments: map[string]any{"name": "Engineering", "taskPrefix": "ENG"},
			Method: http.MethodPost, Path: "/api/gentasks/v1/boards", Body: map[string]any{"name": "Engineering", "taskPrefix": "ENG"},
		},
		{
			Tool: "gentasks_update_board", Arguments: map[string]any{"boardToken": "brd_1", "taskPrefix": "OPS"},
			Method: http.MethodPatch, Path: "/api/gentasks/v1/boards/brd_1", Body: map[string]any{"taskPrefix": "OPS"},
		},
		{
			Tool: "gentasks_delete_board", Arguments: map[string]any{"boardToken": "brd_1"},
			Method: http.MethodDelete, Path: "/api/gentasks/v1/boards/brd_1", Confirmation: "board deleted",
		},
		{
			Tool: "gentasks_add_column", Arguments: map[string]any{"boardToken": "brd_1", "name": "Review"},
			Method: http.MethodPost, Path: "/api/gentasks/v1/boards/brd_1/columns", Body: map[string]any{"name": "Review"},
		},
		{
			Tool: "gentasks_update_column", Arguments: map[string]any{"columnToken": "col_1", "name": "QA"},
			Method: http.MethodPatch, Path: "/api/gentasks/v1/columns/col_1", Body: map[string]any{"name": "QA"},
		},
		{
			Tool: "gentasks_update_column", Arguments: map[string]any{"columnToken": "col_1", "clearName": true},
			Method: http.MethodPatch, Path: "/api/gentasks/v1/columns/col_1", Body: map[string]any{"name": nil},
		},
		{
			Tool: "gentasks_move_column", Arguments: map[string]any{"columnToken": "col_1", "position": 0},
			Method: http.MethodPost, Path: "/api/gentasks/v1/columns/col_1/move", Body: map[string]any{"position": 0}, Confirmation: "column moved",
		},
		{
			Tool: "gentasks_delete_column", Arguments: map[string]any{"columnToken": "col_1"},
			Method: http.MethodDelete, Path: "/api/gentasks/v1/columns/col_1", Confirmation: "column deleted",
		},
		{Tool: "gentasks_list_labels", Arguments: map[string]any{"boardToken": "brd_1"}, Method: http.MethodGet, Path: "/api/gentasks/v1/boards/brd_1/labels"},
		{
			Tool: "gentasks_create_label", Arguments: map[string]any{"boardToken": "brd_1", "name": "bug", "color": "red"},
			Method: http.MethodPost, Path: "/api/gentasks/v1/boards/brd_1/labels", Body: map[string]any{"name": "bug", "color": "red"},
		},
		{
			Tool: "gentasks_update_label", Arguments: map[string]any{"labelToken": "lbl_1", "color": "blue"},
			Method: http.MethodPatch, Path: "/api/gentasks/v1/labels/lbl_1", Body: map[string]any{"color": "blue"},
		},
		{
			Tool: "gentasks_delete_label", Arguments: map[string]any{"labelToken": "lbl_1"},
			Method: http.MethodDelete, Path: "/api/gentasks/v1/labels/lbl_1", Confirmation: "label deleted",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.Name(), func(t *testing.T) {
			toolsettest.ExpectToolRequest(t, gentasks.NewToolset, testCase)
		})
	}
}

func TestGentasksTaskToolsSendTheirRequestToTheModule(t *testing.T) {
	cases := []toolsettest.ToolRequestCase{
		{Tool: "gentasks_list_tasks", Arguments: map[string]any{"boardToken": "brd_1"}, Method: http.MethodGet, Path: "/api/gentasks/v1/boards/brd_1/tasks"},
		{
			Tool: "gentasks_list_tasks", Arguments: map[string]any{"boardToken": "brd_1", "archived": true},
			Method: http.MethodGet, Path: "/api/gentasks/v1/boards/brd_1/tasks", Query: "archived=true",
		},
		{Tool: "gentasks_list_my_assigned_tasks", Method: http.MethodGet, Path: "/api/gentasks/v1/me/assigned-tasks"},
		{
			Tool: "gentasks_list_my_assigned_tasks", Arguments: map[string]any{"limit": 5},
			Method: http.MethodGet, Path: "/api/gentasks/v1/me/assigned-tasks", Query: "limit=5",
		},
		{Tool: "gentasks_get_task", Arguments: map[string]any{"taskToken": "tsk_1"}, Method: http.MethodGet, Path: "/api/gentasks/v1/tasks/tsk_1"},
		{
			Tool: "gentasks_create_task", Arguments: map[string]any{"boardToken": "brd_1", "title": "Ship it"},
			Method: http.MethodPost, Path: "/api/gentasks/v1/boards/brd_1/tasks", Body: map[string]any{"title": "Ship it"},
		},
		{
			Tool: "gentasks_create_task",
			Arguments: map[string]any{
				"boardToken": "brd_1", "title": "Ship it", "description": "Soon", "columnToken": "col_1",
				"assigneeToken": "usr_1", "labelTokens": []string{"lbl_1"}, "urgency": "high",
			},
			Method: http.MethodPost, Path: "/api/gentasks/v1/boards/brd_1/tasks",
			Body: map[string]any{
				"title": "Ship it", "description": "Soon", "columnToken": "col_1",
				"assigneeToken": "usr_1", "labelTokens": []string{"lbl_1"}, "urgency": "high",
			},
		},
		{
			Tool: "gentasks_update_task", Arguments: map[string]any{"taskToken": "tsk_1", "unassign": true, "clearDescription": true},
			Method: http.MethodPatch, Path: "/api/gentasks/v1/tasks/tsk_1", Body: map[string]any{"assigneeToken": nil, "description": nil},
		},
		{
			Tool: "gentasks_move_task", Arguments: map[string]any{"taskToken": "tsk_1", "columnToken": "col_done", "position": 2},
			Method: http.MethodPost, Path: "/api/gentasks/v1/tasks/tsk_1/move",
			Body: map[string]any{"columnToken": "col_done", "position": 2}, Confirmation: "task moved",
		},
		{
			Tool: "gentasks_archive_task", Arguments: map[string]any{"taskToken": "tsk_1"},
			Method: http.MethodPost, Path: "/api/gentasks/v1/tasks/tsk_1/archive", Confirmation: "task archived",
		},
		{
			Tool: "gentasks_unarchive_task", Arguments: map[string]any{"taskToken": "tsk_1"},
			Method: http.MethodPost, Path: "/api/gentasks/v1/tasks/tsk_1/unarchive", Confirmation: "task restored",
		},
		{
			Tool: "gentasks_delete_task", Arguments: map[string]any{"taskToken": "tsk_1"},
			Method: http.MethodDelete, Path: "/api/gentasks/v1/tasks/tsk_1", Confirmation: "task deleted",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.Name(), func(t *testing.T) {
			toolsettest.ExpectToolRequest(t, gentasks.NewToolset, testCase)
		})
	}
}

func TestTaskPatchBodySendsOnlyTheFieldsTheCallerSent(t *testing.T) {
	title, urgency := "New title", "urgent"

	body, err := gentasks.NewTaskPatchBody(gentasks.UpdateTaskInput{TaskToken: "tsk_1", Title: &title, Urgency: &urgency, LabelTokens: []string{}})

	want := gentasks.RequestBody{"title": "New title", "urgency": "urgent", "labelTokens": []string{}}
	if err != nil || !reflect.DeepEqual(body, want) {
		t.Fatalf("body = %v, err = %v", body, err)
	}
}

func TestTaskPatchBodyRejectsContradictoryFields(t *testing.T) {
	value := "x"
	if _, err := gentasks.NewTaskPatchBody(gentasks.UpdateTaskInput{Description: &value, ClearDescription: true}); !errors.Is(err, gentasks.ErrDescriptionConflict) {
		t.Fatalf("description conflict err = %v", err)
	}
	if _, err := gentasks.NewTaskPatchBody(gentasks.UpdateTaskInput{AssigneeToken: &value, Unassign: true}); !errors.Is(err, gentasks.ErrAssigneeConflict) {
		t.Fatalf("assignee conflict err = %v", err)
	}
}

func TestContradictoryUpdatesFailWithoutCallingTheModule(t *testing.T) {
	api, requester := toolsettest.NewFakeAPIRequester(t)
	session := toolsettest.ConnectWithFullAccess(t, gentasks.NewToolset(requester))

	taskResult := toolsettest.CallTool(t, session, "gentasks_update_task", map[string]any{"taskToken": "tsk_1", "assigneeToken": "usr_1", "unassign": true})
	columnResult := toolsettest.CallTool(t, session, "gentasks_update_column", map[string]any{"columnToken": "col_1", "name": "QA", "clearName": true})

	if !taskResult.IsError || !strings.Contains(toolsettest.ResultText(t, taskResult), "unassign") {
		t.Fatalf("task result = %+v", taskResult)
	}
	if !columnResult.IsError || !strings.Contains(toolsettest.ResultText(t, columnResult), "clearName") {
		t.Fatalf("column result = %+v", columnResult)
	}
	if len(api.Requests()) != 0 {
		t.Fatalf("requests = %d, want none", len(api.Requests()))
	}
}

func TestGentasksToolsOnlyAcceptTheModuleEnums(t *testing.T) {
	api, requester := toolsettest.NewFakeAPIRequester(t)
	session := toolsettest.ConnectWithFullAccess(t, gentasks.NewToolset(requester))

	urgency := toolsettest.CallTool(t, session, "gentasks_create_task", map[string]any{"boardToken": "brd_1", "title": "x", "urgency": "asap"})
	color := toolsettest.CallTool(t, session, "gentasks_create_label", map[string]any{"boardToken": "brd_1", "name": "x", "color": "black"})

	if !urgency.IsError || !color.IsError || len(api.Requests()) != 0 {
		t.Fatalf("urgency error = %v, color error = %v, requests = %d", urgency.IsError, color.IsError, len(api.Requests()))
	}
}

func TestGentasksToolsetBelongsToTheModuleAndNeedsItsPermissions(t *testing.T) {
	set := gentasks.NewToolset(nil)
	if set.ModuleID != gentasks.ModuleID {
		t.Fatalf("module = %q", set.ModuleID)
	}
	allowed := map[string]bool{
		gentasks.PermissionBoardRead: true, gentasks.PermissionBoardManage: true,
		gentasks.PermissionTaskWrite: true, gentasks.PermissionTaskDelete: true,
	}
	for _, tool := range set.Tools {
		if !allowed[tool.Permission] || !strings.HasPrefix(tool.Definition.Name, "gentasks_") {
			t.Fatalf("tool %s needs %q", tool.Definition.Name, tool.Permission)
		}
	}
}
