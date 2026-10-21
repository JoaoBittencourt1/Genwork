package gentasks

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/toolset"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	ErrDescriptionConflict = errors.New("send either description or clearDescription, not both")
	ErrAssigneeConflict    = errors.New("send either assigneeToken or unassign, not both")
)

type ListTasksInput struct {
	BoardToken string `json:"boardToken" jsonschema:"token of the board"`
	Archived   bool   `json:"archived,omitempty" jsonschema:"true to list the archived tasks instead of the active ones"`
}

type ListAssignedTasksInput struct {
	Limit int `json:"limit,omitempty" jsonschema:"how many tasks to answer, from 1 to 50; 20 when omitted"`
}

type TaskInput struct {
	TaskToken string `json:"taskToken" jsonschema:"token of the task, from gentasks_get_board or gentasks_list_tasks"`
}

type CreateTaskInput struct {
	BoardToken    string   `json:"boardToken" jsonschema:"token of the board"`
	Title         string   `json:"title" jsonschema:"task title"`
	Description   *string  `json:"description,omitempty" jsonschema:"task description in Markdown"`
	ColumnToken   *string  `json:"columnToken,omitempty" jsonschema:"column to create the task in; the board's first column when omitted"`
	AssigneeToken *string  `json:"assigneeToken,omitempty" jsonschema:"user token of the assignee, from core_list_users"`
	LabelTokens   []string `json:"labelTokens,omitempty" jsonschema:"tokens of labels of the same board"`
	Urgency       *string  `json:"urgency,omitempty" jsonschema:"task urgency; none when omitted"`
}

type UpdateTaskInput struct {
	TaskToken        string   `json:"taskToken" jsonschema:"token of the task"`
	Title            *string  `json:"title,omitempty" jsonschema:"new title; unchanged when omitted"`
	Description      *string  `json:"description,omitempty" jsonschema:"new description in Markdown; unchanged when omitted"`
	ClearDescription bool     `json:"clearDescription,omitempty" jsonschema:"true to remove the description"`
	AssigneeToken    *string  `json:"assigneeToken,omitempty" jsonschema:"user token of the new assignee; unchanged when omitted"`
	Unassign         bool     `json:"unassign,omitempty" jsonschema:"true to remove the assignee"`
	Urgency          *string  `json:"urgency,omitempty" jsonschema:"new urgency; unchanged when omitted"`
	LabelTokens      []string `json:"labelTokens,omitempty" jsonschema:"replaces every label of the task; an empty list removes them all; unchanged when omitted"`
}

type MoveTaskInput struct {
	TaskToken   string `json:"taskToken" jsonschema:"token of the task"`
	ColumnToken string `json:"columnToken" jsonschema:"token of the target column, on the same board"`
	Position    int    `json:"position" jsonschema:"zero-based position inside the target column"`
}

func (tools *GentasksTools) taskTools() []toolset.Tool {
	urgencyEnum := map[string][]any{"urgency": urgencies}
	return []toolset.Tool{
		toolset.NewTool(&mcp.Tool{
			Name: "gentasks_list_tasks", Title: "List tasks", Annotations: toolset.ReadOnly(),
			Description: "Lists the active tasks of a board, or its archived tasks.",
		}, PermissionBoardRead, nil, tools.listTasks),
		toolset.NewTool(&mcp.Tool{
			Name: "gentasks_list_my_assigned_tasks", Title: "List my assigned tasks", Annotations: toolset.ReadOnly(),
			Description: "Lists the active tasks assigned to the signed-in user across every board, longest-assigned first.",
		}, PermissionBoardRead, nil, tools.listMyAssignedTasks),
		toolset.NewTool(&mcp.Tool{
			Name: "gentasks_get_task", Title: "Get task", Annotations: toolset.ReadOnly(),
			Description: "Shows one task with its description, assignee, urgency, and labels.",
		}, PermissionBoardRead, nil, tools.getTask),
		toolset.NewTool(&mcp.Tool{
			Name: "gentasks_create_task", Title: "Create task", Annotations: toolset.Writes(),
			Description: "Creates a task on a board; it gets the next number of the board, such as ENG-12.",
		}, PermissionTaskWrite, urgencyEnum, tools.createTask),
		toolset.NewTool(&mcp.Tool{
			Name: "gentasks_update_task", Title: "Update task", Annotations: toolset.Writes(),
			Description: "Changes a task's title, description, assignee, urgency, or labels; omitted fields stay unchanged.",
		}, PermissionTaskWrite, urgencyEnum, tools.updateTask),
		toolset.NewTool(&mcp.Tool{
			Name: "gentasks_move_task", Title: "Move task", Annotations: toolset.Writes(),
			Description: "Moves a task to a column and position of its board, e.g. to mark it done.",
		}, PermissionTaskWrite, nil, tools.moveTask),
		toolset.NewTool(&mcp.Tool{
			Name: "gentasks_archive_task", Title: "Archive task", Annotations: toolset.Writes(),
			Description: "Archives a task: it leaves the board but can be restored with gentasks_unarchive_task.",
		}, PermissionTaskWrite, nil, tools.archiveTask),
		toolset.NewTool(&mcp.Tool{
			Name: "gentasks_unarchive_task", Title: "Restore archived task", Annotations: toolset.Writes(),
			Description: "Restores an archived task to the board.",
		}, PermissionTaskWrite, nil, tools.unarchiveTask),
		toolset.NewTool(&mcp.Tool{
			Name: "gentasks_delete_task", Title: "Delete task", Annotations: toolset.Destroys(),
			Description: "Deletes a task. Prefer gentasks_archive_task unless the person asks to delete it.",
		}, PermissionTaskDelete, nil, tools.deleteTask),
	}
}

func (tools *GentasksTools) listTasks(ctx context.Context, input ListTasksInput) (any, error) {
	request := platform.APIRequest{Method: http.MethodGet, Path: platform.ExpandPath(apiPrefix+"/boards/%s/tasks", input.BoardToken)}
	if input.Archived {
		request.Query = url.Values{"archived": {"true"}}
	}
	return toolset.Forward(ctx, tools.api, request, "")
}

func (tools *GentasksTools) listMyAssignedTasks(ctx context.Context, input ListAssignedTasksInput) (any, error) {
	request := platform.APIRequest{Method: http.MethodGet, Path: apiPrefix + "/me/assigned-tasks"}
	if input.Limit != 0 {
		request.Query = url.Values{"limit": {strconv.Itoa(input.Limit)}}
	}
	return toolset.Forward(ctx, tools.api, request, "")
}

func (tools *GentasksTools) getTask(ctx context.Context, input TaskInput) (any, error) {
	return tools.send(ctx, http.MethodGet, platform.ExpandPath(apiPrefix+"/tasks/%s", input.TaskToken), nil, "")
}

func (tools *GentasksTools) createTask(ctx context.Context, input CreateTaskInput) (any, error) {
	body := RequestBody{"title": input.Title}
	body.setIfPresent("description", input.Description)
	body.setIfPresent("columnToken", input.ColumnToken)
	body.setIfPresent("assigneeToken", input.AssigneeToken)
	body.setIfPresent("urgency", input.Urgency)
	if input.LabelTokens != nil {
		body["labelTokens"] = input.LabelTokens
	}
	return tools.send(ctx, http.MethodPost, platform.ExpandPath(apiPrefix+"/boards/%s/tasks", input.BoardToken), body, "")
}

func (tools *GentasksTools) updateTask(ctx context.Context, input UpdateTaskInput) (any, error) {
	body, err := NewTaskPatchBody(input)
	if err != nil {
		return nil, err
	}
	return tools.send(ctx, http.MethodPatch, platform.ExpandPath(apiPrefix+"/tasks/%s", input.TaskToken), body, "")
}

func NewTaskPatchBody(input UpdateTaskInput) (RequestBody, error) {
	if input.ClearDescription && input.Description != nil {
		return nil, ErrDescriptionConflict
	}
	if input.Unassign && input.AssigneeToken != nil {
		return nil, ErrAssigneeConflict
	}
	body := RequestBody{}
	body.setIfPresent("title", input.Title)
	body.setIfPresent("description", input.Description)
	body.setIfPresent("assigneeToken", input.AssigneeToken)
	body.setIfPresent("urgency", input.Urgency)
	if input.ClearDescription {
		body["description"] = nil
	}
	if input.Unassign {
		body["assigneeToken"] = nil
	}
	if input.LabelTokens != nil {
		body["labelTokens"] = input.LabelTokens
	}
	return body, nil
}

func (tools *GentasksTools) moveTask(ctx context.Context, input MoveTaskInput) (any, error) {
	path := platform.ExpandPath(apiPrefix+"/tasks/%s/move", input.TaskToken)
	return tools.send(ctx, http.MethodPost, path, map[string]any{"columnToken": input.ColumnToken, "position": input.Position}, "task moved")
}

func (tools *GentasksTools) archiveTask(ctx context.Context, input TaskInput) (any, error) {
	return tools.send(ctx, http.MethodPost, platform.ExpandPath(apiPrefix+"/tasks/%s/archive", input.TaskToken), nil, "task archived")
}

func (tools *GentasksTools) unarchiveTask(ctx context.Context, input TaskInput) (any, error) {
	return tools.send(ctx, http.MethodPost, platform.ExpandPath(apiPrefix+"/tasks/%s/unarchive", input.TaskToken), nil, "task restored")
}

func (tools *GentasksTools) deleteTask(ctx context.Context, input TaskInput) (any, error) {
	return tools.send(ctx, http.MethodDelete, platform.ExpandPath(apiPrefix+"/tasks/%s", input.TaskToken), nil, "task deleted")
}
