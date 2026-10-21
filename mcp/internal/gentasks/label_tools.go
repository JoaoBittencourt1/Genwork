package gentasks

import (
	"context"
	"net/http"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/toolset"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type CreateLabelInput struct {
	BoardToken string `json:"boardToken" jsonschema:"token of the board"`
	Name       string `json:"name" jsonschema:"label name, unique on the board"`
	Color      string `json:"color" jsonschema:"label color"`
}

type LabelInput struct {
	LabelToken string `json:"labelToken" jsonschema:"token of the label, from gentasks_list_labels"`
}

type UpdateLabelInput struct {
	LabelToken string  `json:"labelToken" jsonschema:"token of the label"`
	Name       *string `json:"name,omitempty" jsonschema:"new label name; unchanged when omitted"`
	Color      *string `json:"color,omitempty" jsonschema:"new label color; unchanged when omitted"`
}

func (tools *GentasksTools) labelTools() []toolset.Tool {
	colorEnum := map[string][]any{"color": labelColors}
	return []toolset.Tool{
		toolset.NewTool(&mcp.Tool{
			Name: "gentasks_list_labels", Title: "List labels", Annotations: toolset.ReadOnly(),
			Description: "Lists the labels of a board.",
		}, PermissionBoardRead, nil, tools.listLabels),
		toolset.NewTool(&mcp.Tool{
			Name: "gentasks_create_label", Title: "Create label", Annotations: toolset.Writes(),
			Description: "Creates a label on a board.",
		}, PermissionBoardManage, colorEnum, tools.createLabel),
		toolset.NewTool(&mcp.Tool{
			Name: "gentasks_update_label", Title: "Update label", Annotations: toolset.Writes(),
			Description: "Renames a label or changes its color; omitted fields stay unchanged.",
		}, PermissionBoardManage, colorEnum, tools.updateLabel),
		toolset.NewTool(&mcp.Tool{
			Name: "gentasks_delete_label", Title: "Delete label", Annotations: toolset.Destroys(),
			Description: "Deletes a label and removes it from every task.",
		}, PermissionBoardManage, nil, tools.deleteLabel),
	}
}

func (tools *GentasksTools) listLabels(ctx context.Context, input BoardInput) (any, error) {
	return tools.send(ctx, http.MethodGet, platform.ExpandPath(apiPrefix+"/boards/%s/labels", input.BoardToken), nil, "")
}

func (tools *GentasksTools) createLabel(ctx context.Context, input CreateLabelInput) (any, error) {
	path := platform.ExpandPath(apiPrefix+"/boards/%s/labels", input.BoardToken)
	return tools.send(ctx, http.MethodPost, path, map[string]string{"name": input.Name, "color": input.Color}, "")
}

func (tools *GentasksTools) updateLabel(ctx context.Context, input UpdateLabelInput) (any, error) {
	body := RequestBody{}
	body.setIfPresent("name", input.Name)
	body.setIfPresent("color", input.Color)
	return tools.send(ctx, http.MethodPatch, platform.ExpandPath(apiPrefix+"/labels/%s", input.LabelToken), body, "")
}

func (tools *GentasksTools) deleteLabel(ctx context.Context, input LabelInput) (any, error) {
	return tools.send(ctx, http.MethodDelete, platform.ExpandPath(apiPrefix+"/labels/%s", input.LabelToken), nil, "label deleted")
}
