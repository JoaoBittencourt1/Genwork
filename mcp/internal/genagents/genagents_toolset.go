// Package genagents exposes the genagents module's v1 API as MCP tools (rule 59: one folder per module): the bots, their
// runs, and the task conversations, so the admin chat reaches them like every other module.
package genagents

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/toolset"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	ModuleID  = "genagents"
	apiPrefix = "/api/genagents/v1"

	PermissionBotRead  = "genagents.bot.read"
	PermissionRunRead  = "genagents.run.read"
	PermissionRunWrite = "genagents.run.write"
)

var botKinds = []any{"dev", "reviewer", "assistant"}

type ListBotsInput struct {
	Kind string `json:"kind,omitempty" jsonschema:"only bots of this kind: dev, reviewer, or assistant"`
}

type ListRunsInput struct {
	ActiveOnly bool `json:"activeOnly,omitempty" jsonschema:"true to list only runs that are queued, running, or waiting for input"`
	Limit      int  `json:"limit,omitempty" jsonschema:"how many runs to return, newest first, between 1 and 200"`
}

type TaskConversationInput struct {
	TaskToken     string `json:"taskToken" jsonschema:"token of the task, from gentasks_get_task"`
	AfterSequence int    `json:"afterSequence,omitempty" jsonschema:"only entries after this sequence number; omit to read from the start"`
}

type PostTaskMessageInput struct {
	TaskToken string `json:"taskToken" jsonschema:"token of the task whose bot conversation receives the message"`
	Text      string `json:"text" jsonschema:"the message, up to 8000 characters, delivered to the bot working the task"`
}

type RequestBotReviewInput struct {
	TaskToken string  `json:"taskToken" jsonschema:"token of the task whose linked open pull requests are reviewed"`
	BotToken  *string `json:"botToken,omitempty" jsonschema:"token of the reviewer bot, from genagents_list_bots; omit when only one reviewer bot is enabled"`
}

type GenagentsTools struct {
	api platform.APIRequester
}

func NewToolset(api platform.APIRequester) toolset.Toolset {
	tools := &GenagentsTools{api: api}
	return toolset.Toolset{ModuleID: ModuleID, Tools: []toolset.Tool{
		toolset.NewTool(&mcp.Tool{
			Name: "genagents_list_bots", Title: "List bots", Annotations: toolset.ReadOnly(),
			Description: "Lists the AI bots with their kind, provider, model, and member token; never credentials.",
		}, PermissionBotRead, map[string][]any{"kind": botKinds}, tools.listBots),
		toolset.NewTool(&mcp.Tool{
			Name: "genagents_list_runs", Title: "List bot runs", Annotations: toolset.ReadOnly(),
			Description: "Lists the most recent bot runs with their state and the task each one works on.",
		}, PermissionRunRead, nil, tools.listRuns),
		toolset.NewTool(&mcp.Tool{
			Name: "genagents_read_task_conversation", Title: "Read bot conversation", Annotations: toolset.ReadOnly(),
			Description: "Reads the conversation of a task with the bots that work on it, and its active run.",
		}, PermissionRunRead, nil, tools.readTaskConversation),
		toolset.NewTool(&mcp.Tool{
			Name: "genagents_post_task_message", Title: "Message a bot", Annotations: toolset.Writes(),
			Description: "Posts a message to the conversation of a task; the bot working on it receives it.",
		}, PermissionRunWrite, nil, tools.postTaskMessage),
		toolset.NewTool(&mcp.Tool{
			Name: "genagents_request_bot_review", Title: "Request bot review", Annotations: toolset.Writes(),
			Description: "Asks a reviewer bot to review the open and draft pull requests linked to a task; the review appears on GitHub and in the task conversation.",
		}, PermissionRunWrite, nil, tools.requestBotReview),
	}}
}

func (tools *GenagentsTools) send(ctx context.Context, request platform.APIRequest, confirmation string) (any, error) {
	return toolset.Forward(ctx, tools.api, request, confirmation)
}

func (tools *GenagentsTools) listBots(ctx context.Context, input ListBotsInput) (any, error) {
	request := platform.APIRequest{Method: http.MethodGet, Path: apiPrefix + "/bots"}
	if input.Kind != "" {
		request.Query = url.Values{"kind": {input.Kind}}
	}
	return tools.send(ctx, request, "")
}

func (tools *GenagentsTools) listRuns(ctx context.Context, input ListRunsInput) (any, error) {
	query := url.Values{}
	if input.ActiveOnly {
		query.Set("state", "active")
	}
	if input.Limit > 0 {
		query.Set("limit", strconv.Itoa(input.Limit))
	}
	return tools.send(ctx, platform.APIRequest{Method: http.MethodGet, Path: apiPrefix + "/runs", Query: query}, "")
}

func (tools *GenagentsTools) readTaskConversation(ctx context.Context, input TaskConversationInput) (any, error) {
	request := platform.APIRequest{Method: http.MethodGet, Path: platform.ExpandPath(apiPrefix+"/tasks/%s/conversation", input.TaskToken)}
	if input.AfterSequence > 0 {
		request.Query = url.Values{"after": {strconv.Itoa(input.AfterSequence)}}
	}
	return tools.send(ctx, request, "")
}

func (tools *GenagentsTools) postTaskMessage(ctx context.Context, input PostTaskMessageInput) (any, error) {
	request := platform.APIRequest{
		Method: http.MethodPost, Path: platform.ExpandPath(apiPrefix+"/tasks/%s/conversation/messages", input.TaskToken), Body: map[string]string{"text": input.Text},
	}
	return tools.send(ctx, request, "message posted")
}

func (tools *GenagentsTools) requestBotReview(ctx context.Context, input RequestBotReviewInput) (any, error) {
	body := map[string]any{}
	if input.BotToken != nil {
		body["botToken"] = *input.BotToken
	}
	request := platform.APIRequest{Method: http.MethodPost, Path: platform.ExpandPath(apiPrefix+"/tasks/%s/bot-reviews", input.TaskToken), Body: body}
	return tools.send(ctx, request, "")
}
