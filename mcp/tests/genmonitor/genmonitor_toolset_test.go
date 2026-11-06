package genmonitor_test

import (
	"net/http"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/genmonitor"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/toolset/toolsettest"
)

func TestGenmonitorHostToolSendsItsRequestToTheModule(t *testing.T) {
	toolsettest.ExpectToolRequest(t, genmonitor.NewToolset, toolsettest.ToolRequestCase{
		Tool: "genmonitor_get_host", Method: http.MethodGet, Path: "/api/genmonitor/v1/host",
	})
}

func TestGenmonitorToolsNeedTheHostPermission(t *testing.T) {
	tools := genmonitor.NewToolset(nil)

	if tools.ModuleID != "genmonitor" || len(tools.Tools) != 1 || tools.Tools[0].Permission != "genmonitor.host.read" {
		t.Errorf("toolset = %+v", tools)
	}
}
