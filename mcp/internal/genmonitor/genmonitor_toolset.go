// Package genmonitor exposes the genmonitor module's v1 API as MCP tools (rule 59: one folder per module).
package genmonitor

import (
	"context"
	"net/http"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/toolset"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	ModuleID           = "genmonitor"
	apiPrefix          = "/api/genmonitor/v1"
	PermissionHostRead = "genmonitor.host.read"
)

type GenmonitorTools struct {
	api platform.APIRequester
}

func NewToolset(api platform.APIRequester) toolset.Toolset {
	tools := &GenmonitorTools{api: api}
	return toolset.Toolset{ModuleID: ModuleID, Tools: []toolset.Tool{
		toolset.NewTool(&mcp.Tool{
			Name: "genmonitor_get_host", Title: "Get host metrics", Annotations: toolset.ReadOnly(),
			Description: "Reads the latest snapshot of the machine running this installation: CPU, memory, disk, network, and operating system.",
		}, PermissionHostRead, nil, tools.getHost),
	}}
}

func (tools *GenmonitorTools) getHost(ctx context.Context, _ toolset.NoInput) (any, error) {
	return toolset.Forward(ctx, tools.api, platform.APIRequest{Method: http.MethodGet, Path: apiPrefix + "/host"}, "")
}
