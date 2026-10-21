// Package gentasks exposes the gentasks module's v1 API as MCP tools (rule 59: one folder per module).
package gentasks

import (
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/toolset"
)

const ModuleID = "gentasks"

const apiPrefix = "/api/gentasks/v1"

const (
	PermissionBoardRead   = "gentasks.board.read"
	PermissionBoardManage = "gentasks.board.manage"
	PermissionTaskWrite   = "gentasks.task.write"
	PermissionTaskDelete  = "gentasks.task.delete"
)

var (
	urgencies   = []any{"none", "low", "medium", "high", "urgent"}
	labelColors = []any{"gray", "red", "orange", "yellow", "green", "teal", "blue", "purple", "pink"}
)

type GentasksTools struct {
	api platform.APIRequester
}

func NewToolset(api platform.APIRequester) toolset.Toolset {
	tools := &GentasksTools{api: api}
	var all []toolset.Tool
	all = append(all, tools.boardTools()...)
	all = append(all, tools.labelTools()...)
	all = append(all, tools.taskTools()...)
	return toolset.Toolset{ModuleID: ModuleID, Tools: all}
}

// RequestBody carries only the fields the caller sent; in a PATCH, a nil value clears a nullable field (rule 17).
type RequestBody map[string]any

func (body RequestBody) setIfPresent(field string, value *string) {
	if value != nil {
		body[field] = *value
	}
}
