package gentasks

import (
	"context"
	"errors"
	"net/http"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/toolset"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var ErrColumnNameConflict = errors.New("send either name or clearName, not both")

type BoardInput struct {
	BoardToken string `json:"boardToken" jsonschema:"token of the board, from gentasks_list_boards"`
}

type CreateBoardInput struct {
	Name       string  `json:"name" jsonschema:"board name"`
	TaskPrefix *string `json:"taskPrefix,omitempty" jsonschema:"prefix of task numbers such as ENG for ENG-12: a letter then up to 9 letters or digits; derived from the name when omitted"`
}

type UpdateBoardInput struct {
	BoardToken string  `json:"boardToken" jsonschema:"token of the board"`
	Name       *string `json:"name,omitempty" jsonschema:"new board name; unchanged when omitted"`
	TaskPrefix *string `json:"taskPrefix,omitempty" jsonschema:"new task number prefix; unchanged when omitted"`
}

type AddColumnInput struct {
	BoardToken string `json:"boardToken" jsonschema:"token of the board"`
	Name       string `json:"name" jsonschema:"column name"`
}

type ColumnInput struct {
	ColumnToken string `json:"columnToken" jsonschema:"token of the column, from gentasks_get_board"`
}

type UpdateColumnInput struct {
	ColumnToken string  `json:"columnToken" jsonschema:"token of the column"`
	Name        *string `json:"name,omitempty" jsonschema:"new column name"`
	ClearName   bool    `json:"clearName,omitempty" jsonschema:"true to drop a custom name, so a preset column (backlog, todo, done) shows its default name again"`
}

type MoveColumnInput struct {
	ColumnToken string `json:"columnToken" jsonschema:"token of the column"`
	Position    int    `json:"position" jsonschema:"zero-based position among the board's columns"`
}

func (tools *GentasksTools) boardTools() []toolset.Tool {
	return []toolset.Tool{
		toolset.NewTool(&mcp.Tool{
			Name: "gentasks_list_boards", Title: "List boards", Annotations: toolset.ReadOnly(),
			Description: "Lists the task boards of this workspace.",
		}, PermissionBoardRead, nil, tools.listBoards),
		toolset.NewTool(&mcp.Tool{
			Name: "gentasks_get_board", Title: "Get board", Annotations: toolset.ReadOnly(),
			Description: "Shows a board with its columns, in order, and the active tasks of each column.",
		}, PermissionBoardRead, nil, tools.getBoard),
		toolset.NewTool(&mcp.Tool{
			Name: "gentasks_create_board", Title: "Create board", Annotations: toolset.Writes(),
			Description: "Creates a board with the preset columns backlog, todo, and done.",
		}, PermissionBoardManage, nil, tools.createBoard),
		toolset.NewTool(&mcp.Tool{
			Name: "gentasks_update_board", Title: "Update board", Annotations: toolset.Writes(),
			Description: "Renames a board or changes its task number prefix; omitted fields stay unchanged.",
		}, PermissionBoardManage, nil, tools.updateBoard),
		toolset.NewTool(&mcp.Tool{
			Name: "gentasks_delete_board", Title: "Delete board", Annotations: toolset.Destroys(),
			Description: "Deletes a board; its columns, labels, and tasks are no longer listed.",
		}, PermissionBoardManage, nil, tools.deleteBoard),
		toolset.NewTool(&mcp.Tool{
			Name: "gentasks_add_column", Title: "Add column", Annotations: toolset.Writes(),
			Description: "Adds a column at the end of a board.",
		}, PermissionBoardManage, nil, tools.addColumn),
		toolset.NewTool(&mcp.Tool{
			Name: "gentasks_update_column", Title: "Rename column", Annotations: toolset.Writes(),
			Description: "Renames a column, or drops the custom name of a preset column.",
		}, PermissionBoardManage, nil, tools.updateColumn),
		toolset.NewTool(&mcp.Tool{
			Name: "gentasks_move_column", Title: "Move column", Annotations: toolset.Writes(),
			Description: "Moves a column to another position on its board.",
		}, PermissionBoardManage, nil, tools.moveColumn),
		toolset.NewTool(&mcp.Tool{
			Name: "gentasks_delete_column", Title: "Delete column", Annotations: toolset.Destroys(),
			Description: "Deletes an empty column; a board keeps at least one column, so move its tasks out first.",
		}, PermissionBoardManage, nil, tools.deleteColumn),
	}
}

func (tools *GentasksTools) listBoards(ctx context.Context, _ toolset.NoInput) (any, error) {
	return tools.send(ctx, http.MethodGet, apiPrefix+"/boards", nil, "")
}

func (tools *GentasksTools) getBoard(ctx context.Context, input BoardInput) (any, error) {
	return tools.send(ctx, http.MethodGet, platform.ExpandPath(apiPrefix+"/boards/%s", input.BoardToken), nil, "")
}

func (tools *GentasksTools) createBoard(ctx context.Context, input CreateBoardInput) (any, error) {
	body := RequestBody{"name": input.Name}
	body.setIfPresent("taskPrefix", input.TaskPrefix)
	return tools.send(ctx, http.MethodPost, apiPrefix+"/boards", body, "")
}

func (tools *GentasksTools) updateBoard(ctx context.Context, input UpdateBoardInput) (any, error) {
	body := RequestBody{}
	body.setIfPresent("name", input.Name)
	body.setIfPresent("taskPrefix", input.TaskPrefix)
	return tools.send(ctx, http.MethodPatch, platform.ExpandPath(apiPrefix+"/boards/%s", input.BoardToken), body, "")
}

func (tools *GentasksTools) deleteBoard(ctx context.Context, input BoardInput) (any, error) {
	return tools.send(ctx, http.MethodDelete, platform.ExpandPath(apiPrefix+"/boards/%s", input.BoardToken), nil, "board deleted")
}

func (tools *GentasksTools) addColumn(ctx context.Context, input AddColumnInput) (any, error) {
	path := platform.ExpandPath(apiPrefix+"/boards/%s/columns", input.BoardToken)
	return tools.send(ctx, http.MethodPost, path, map[string]string{"name": input.Name}, "")
}

func (tools *GentasksTools) updateColumn(ctx context.Context, input UpdateColumnInput) (any, error) {
	if input.ClearName && input.Name != nil {
		return nil, ErrColumnNameConflict
	}
	body := RequestBody{}
	body.setIfPresent("name", input.Name)
	if input.ClearName {
		body["name"] = nil
	}
	return tools.send(ctx, http.MethodPatch, platform.ExpandPath(apiPrefix+"/columns/%s", input.ColumnToken), body, "")
}

func (tools *GentasksTools) moveColumn(ctx context.Context, input MoveColumnInput) (any, error) {
	path := platform.ExpandPath(apiPrefix+"/columns/%s/move", input.ColumnToken)
	return tools.send(ctx, http.MethodPost, path, map[string]int{"position": input.Position}, "column moved")
}

func (tools *GentasksTools) deleteColumn(ctx context.Context, input ColumnInput) (any, error) {
	return tools.send(ctx, http.MethodDelete, platform.ExpandPath(apiPrefix+"/columns/%s", input.ColumnToken), nil, "column deleted")
}

func (tools *GentasksTools) send(ctx context.Context, method, path string, body any, confirmation string) (any, error) {
	return toolset.Forward(ctx, tools.api, platform.APIRequest{Method: method, Path: path, Body: body}, confirmation)
}
