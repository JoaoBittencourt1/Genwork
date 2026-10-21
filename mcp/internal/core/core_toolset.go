package core

import (
	"context"
	"net/http"
	"net/url"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/toolset"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	PermissionInviteCreate = "core.invite.create"
	PermissionInviteRead   = "core.invite.read"
	PermissionInviteRevoke = "core.invite.revoke"
	PermissionUserRead     = "core.user.read"
)

var inviteStatuses = []any{"pending", "accepted", "revoked", "expired"}

type UserInput struct {
	UserToken string `json:"userToken" jsonschema:"token of the user, from core_list_users"`
}

type ListInvitesInput struct {
	Status string `json:"status,omitempty" jsonschema:"only invites in this status; all of them when omitted"`
}

type CreateInviteInput struct {
	Email     string `json:"email" jsonschema:"e-mail address of the person to invite"`
	RoleToken string `json:"roleToken" jsonschema:"token of the role the person gets, from core_list_roles"`
}

type InviteInput struct {
	InviteToken string `json:"inviteToken" jsonschema:"token of the invite, from core_list_invites"`
}

type SessionInput struct {
	SessionToken string `json:"sessionToken" jsonschema:"token of the session, from core_list_sessions"`
}

type CoreTools struct {
	api platform.APIRequester
}

func NewToolset(api platform.APIRequester) toolset.Toolset {
	tools := &CoreTools{api: api}
	return toolset.Toolset{Tools: []toolset.Tool{
		toolset.NewTool(&mcp.Tool{
			Name: "core_get_me", Title: "Get signed-in user", Annotations: toolset.ReadOnly(),
			Description: "Shows the signed-in user: token, name, e-mail, role, and every permission they hold.",
		}, "", nil, tools.getMe),
		toolset.NewTool(&mcp.Tool{
			Name: "core_get_capabilities", Title: "Get instance capabilities", Annotations: toolset.ReadOnly(),
			Description: "Shows the core version and the modules enabled on this Genwork instance, with their versions.",
		}, "", nil, tools.getCapabilities),
		toolset.NewTool(&mcp.Tool{
			Name: "core_list_users", Title: "List users", Annotations: toolset.ReadOnly(),
			Description: "Lists the people of this workspace (token, name, e-mail). Use a user token to assign work to them.",
		}, PermissionUserRead, nil, tools.listUsers),
		toolset.NewTool(&mcp.Tool{
			Name: "core_get_user", Title: "Get user", Annotations: toolset.ReadOnly(),
			Description: "Shows one person of this workspace by user token.",
		}, PermissionUserRead, nil, tools.getUser),
		toolset.NewTool(&mcp.Tool{
			Name: "core_list_roles", Title: "List roles", Annotations: toolset.ReadOnly(),
			Description: "Lists the roles a person can be invited with, and the permissions of each role.",
		}, PermissionInviteCreate, nil, tools.listRoles),
		toolset.NewTool(&mcp.Tool{
			Name: "core_list_invites", Title: "List invites", Annotations: toolset.ReadOnly(),
			Description: "Lists invites to this workspace, optionally only those in one status.",
		}, PermissionInviteRead, map[string][]any{"status": inviteStatuses}, tools.listInvites),
		toolset.NewTool(&mcp.Tool{
			Name: "core_create_invite", Title: "Invite a person", Annotations: toolset.Writes(),
			Description: "Invites a person by e-mail with a role. Answers the invite and the acceptance link; the link is also e-mailed when mail is configured.",
		}, PermissionInviteCreate, nil, tools.createInvite),
		toolset.NewTool(&mcp.Tool{
			Name: "core_resend_invite", Title: "Resend invite", Annotations: toolset.Writes(),
			Description: "Sends a pending invite again with a fresh acceptance link.",
		}, PermissionInviteCreate, nil, tools.resendInvite),
		toolset.NewTool(&mcp.Tool{
			Name: "core_revoke_invite", Title: "Revoke invite", Annotations: toolset.Destroys(),
			Description: "Revokes a pending invite, so its link stops working.",
		}, PermissionInviteRevoke, nil, tools.revokeInvite),
		toolset.NewTool(&mcp.Tool{
			Name: "core_list_sessions", Title: "List my sessions", Annotations: toolset.ReadOnly(),
			Description: "Lists the signed-in user's active sessions (web, desktop, this MCP server), marking the current one.",
		}, "", nil, tools.listSessions),
		toolset.NewTool(&mcp.Tool{
			Name: "core_revoke_session", Title: "Revoke one of my sessions", Annotations: toolset.Destroys(),
			Description: "Signs the signed-in user out of one of their sessions. Revoking the current session signs this MCP server out.",
		}, "", nil, tools.revokeSession),
	}}
}

func (tools *CoreTools) getMe(ctx context.Context, _ toolset.NoInput) (any, error) {
	return tools.get(ctx, "/api/core/v1/me", nil)
}

func (tools *CoreTools) getCapabilities(ctx context.Context, _ toolset.NoInput) (any, error) {
	return tools.get(ctx, "/api/core/v1/capabilities", nil)
}

func (tools *CoreTools) listUsers(ctx context.Context, _ toolset.NoInput) (any, error) {
	return tools.get(ctx, "/api/core/v1/users", nil)
}

func (tools *CoreTools) getUser(ctx context.Context, input UserInput) (any, error) {
	return tools.get(ctx, platform.ExpandPath("/api/core/v1/users/%s", input.UserToken), nil)
}

func (tools *CoreTools) listRoles(ctx context.Context, _ toolset.NoInput) (any, error) {
	return tools.get(ctx, "/api/core/v1/roles", nil)
}

func (tools *CoreTools) listInvites(ctx context.Context, input ListInvitesInput) (any, error) {
	var query url.Values
	if input.Status != "" {
		query = url.Values{"status": {input.Status}}
	}
	return tools.get(ctx, "/api/core/v1/invites", query)
}

func (tools *CoreTools) createInvite(ctx context.Context, input CreateInviteInput) (any, error) {
	return toolset.Forward(ctx, tools.api, platform.APIRequest{
		Method: http.MethodPost, Path: "/api/core/v1/invites",
		Body: map[string]string{"email": input.Email, "role": input.RoleToken},
	}, "")
}

func (tools *CoreTools) resendInvite(ctx context.Context, input InviteInput) (any, error) {
	return toolset.Forward(ctx, tools.api, platform.APIRequest{
		Method: http.MethodPost, Path: platform.ExpandPath("/api/core/v1/invites/%s/resend", input.InviteToken),
	}, "")
}

func (tools *CoreTools) revokeInvite(ctx context.Context, input InviteInput) (any, error) {
	return toolset.Forward(ctx, tools.api, platform.APIRequest{
		Method: http.MethodDelete, Path: platform.ExpandPath("/api/core/v1/invites/%s", input.InviteToken),
	}, "invite revoked")
}

func (tools *CoreTools) listSessions(ctx context.Context, _ toolset.NoInput) (any, error) {
	return tools.get(ctx, "/api/core/v1/auth/sessions", nil)
}

func (tools *CoreTools) revokeSession(ctx context.Context, input SessionInput) (any, error) {
	return toolset.Forward(ctx, tools.api, platform.APIRequest{
		Method: http.MethodDelete, Path: platform.ExpandPath("/api/core/v1/auth/sessions/%s", input.SessionToken),
	}, "session revoked")
}

func (tools *CoreTools) get(ctx context.Context, path string, query url.Values) (any, error) {
	return toolset.Forward(ctx, tools.api, platform.APIRequest{Method: http.MethodGet, Path: path, Query: query}, "")
}
