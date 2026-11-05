package genagents_test

import (
	"net/http"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/genagents"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/toolset/toolsettest"
)

func TestGenagentsToolsSendTheirRequestToTheModule(t *testing.T) {
	cases := []toolsettest.ToolRequestCase{
		{Tool: "genagents_list_bots", Method: http.MethodGet, Path: "/api/genagents/v1/bots"},
		{Tool: "genagents_list_bots", Arguments: map[string]any{"kind": "reviewer"}, Method: http.MethodGet, Path: "/api/genagents/v1/bots", Query: "kind=reviewer"},
		{Tool: "genagents_list_runs", Method: http.MethodGet, Path: "/api/genagents/v1/runs"},
		{
			Tool: "genagents_list_runs", Arguments: map[string]any{"activeOnly": true, "limit": 10},
			Method: http.MethodGet, Path: "/api/genagents/v1/runs", Query: "limit=10&state=active",
		},
		{Tool: "genagents_read_task_conversation", Arguments: map[string]any{"taskToken": "tsk_1"}, Method: http.MethodGet, Path: "/api/genagents/v1/tasks/tsk_1/conversation"},
		{
			Tool: "genagents_read_task_conversation", Arguments: map[string]any{"taskToken": "tsk_1", "afterSequence": 40},
			Method: http.MethodGet, Path: "/api/genagents/v1/tasks/tsk_1/conversation", Query: "after=40",
		},
		{
			Tool: "genagents_post_task_message", Arguments: map[string]any{"taskToken": "tsk_1", "text": "use the repository"},
			Method: http.MethodPost, Path: "/api/genagents/v1/tasks/tsk_1/conversation/messages", Body: map[string]any{"text": "use the repository"}, Confirmation: "message posted",
		},
		{
			Tool: "genagents_request_bot_review", Arguments: map[string]any{"taskToken": "tsk_1"},
			Method: http.MethodPost, Path: "/api/genagents/v1/tasks/tsk_1/bot-reviews", Body: map[string]any{},
		},
		{
			Tool: "genagents_request_bot_review", Arguments: map[string]any{"taskToken": "tsk_1", "botToken": "bot_1"},
			Method: http.MethodPost, Path: "/api/genagents/v1/tasks/tsk_1/bot-reviews", Body: map[string]any{"botToken": "bot_1"},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.Name(), func(t *testing.T) {
			toolsettest.ExpectToolRequest(t, genagents.NewToolset, testCase)
		})
	}
}

func TestGenagentsToolsAskForTheirOwnPermissions(t *testing.T) {
	permissions := map[string]string{}
	for _, tool := range genagents.NewToolset(nil).Tools {
		permissions[tool.Definition.Name] = tool.Permission
	}

	want := map[string]string{
		"genagents_list_bots": "genagents.bot.read", "genagents_list_runs": "genagents.run.read", "genagents_read_task_conversation": "genagents.run.read",
		"genagents_post_task_message": "genagents.run.write", "genagents_request_bot_review": "genagents.run.write",
	}
	for name, permission := range want {
		if permissions[name] != permission {
			t.Errorf("%s needs %q, want %q", name, permissions[name], permission)
		}
	}
	if len(permissions) != len(want) {
		t.Errorf("tools = %v", permissions)
	}
}
