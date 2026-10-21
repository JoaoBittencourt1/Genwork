package core_test

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/core"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/toolset"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/toolset/toolsettest"
)

func TestCoreToolsSendTheirRequestToCore(t *testing.T) {
	cases := []toolsettest.ToolRequestCase{
		{Tool: "core_get_me", Method: http.MethodGet, Path: "/api/core/v1/me"},
		{Tool: "core_get_capabilities", Method: http.MethodGet, Path: "/api/core/v1/capabilities"},
		{Tool: "core_list_users", Method: http.MethodGet, Path: "/api/core/v1/users"},
		{Tool: "core_get_user", Arguments: map[string]any{"userToken": "usr/1"}, Method: http.MethodGet, Path: "/api/core/v1/users/usr%2F1"},
		{Tool: "core_list_roles", Method: http.MethodGet, Path: "/api/core/v1/roles"},
		{Tool: "core_list_invites", Method: http.MethodGet, Path: "/api/core/v1/invites"},
		{Tool: "core_list_invites", Arguments: map[string]any{"status": "pending"}, Method: http.MethodGet, Path: "/api/core/v1/invites", Query: "status=pending"},
		{
			Tool: "core_create_invite", Arguments: map[string]any{"email": "grace@example.test", "roleToken": "rol_member"},
			Method: http.MethodPost, Path: "/api/core/v1/invites", Body: map[string]any{"email": "grace@example.test", "role": "rol_member"},
		},
		{Tool: "core_resend_invite", Arguments: map[string]any{"inviteToken": "inv_1"}, Method: http.MethodPost, Path: "/api/core/v1/invites/inv_1/resend"},
		{
			Tool: "core_revoke_invite", Arguments: map[string]any{"inviteToken": "inv_1"},
			Method: http.MethodDelete, Path: "/api/core/v1/invites/inv_1", Confirmation: "invite revoked",
		},
		{Tool: "core_list_sessions", Method: http.MethodGet, Path: "/api/core/v1/auth/sessions"},
		{
			Tool: "core_revoke_session", Arguments: map[string]any{"sessionToken": "ses_1"},
			Method: http.MethodDelete, Path: "/api/core/v1/auth/sessions/ses_1", Confirmation: "session revoked",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.Name(), func(t *testing.T) {
			toolsettest.ExpectToolRequest(t, core.NewToolset, testCase)
		})
	}
}

func TestCoreListInvitesRejectsAnUnknownStatusBeforeCallingCore(t *testing.T) {
	api, requester := toolsettest.NewFakeAPIRequester(t)
	session := toolsettest.ConnectWithFullAccess(t, core.NewToolset(requester))

	result := toolsettest.CallTool(t, session, "core_list_invites", map[string]any{"status": "archived"})

	if !result.IsError || len(api.Requests()) != 0 {
		t.Fatalf("isError = %v, requests = %d", result.IsError, len(api.Requests()))
	}
}

func TestCoreToolsetNeedsTheDocumentedPermissions(t *testing.T) {
	permissions := map[string]string{}
	for _, tool := range core.NewToolset(nil).Tools {
		permissions[tool.Definition.Name] = tool.Permission
	}
	want := map[string]string{
		"core_get_me": "", "core_get_capabilities": "", "core_list_sessions": "", "core_revoke_session": "",
		"core_list_users": core.PermissionUserRead, "core_get_user": core.PermissionUserRead,
		"core_list_roles": core.PermissionInviteCreate, "core_create_invite": core.PermissionInviteCreate,
		"core_resend_invite": core.PermissionInviteCreate, "core_list_invites": core.PermissionInviteRead,
		"core_revoke_invite": core.PermissionInviteRevoke,
	}
	if !reflect.DeepEqual(permissions, want) {
		t.Fatalf("permissions = %v", permissions)
	}
}

func TestCoreAccessReaderCombinesCapabilitiesAndPermissions(t *testing.T) {
	api, requester := toolsettest.NewFakeAPIRequester(t)
	api.RespondWithFixture(t, http.MethodGet, "/api/core/v1/capabilities", http.StatusOK, "capabilities_with_gentasks.json")
	api.RespondWithFixture(t, http.MethodGet, "/api/core/v1/me", http.StatusOK, "me_reader.json")

	snapshot, err := core.NewCoreAccessReader(requester).ReadAccess(context.Background())

	want := toolset.AccessSnapshot{EnabledModuleIDs: []string{"gentasks"}, Permissions: []string{"gentasks.board.read"}}
	if err != nil || !reflect.DeepEqual(snapshot, want) {
		t.Fatalf("snapshot = %+v, err = %v", snapshot, err)
	}
}

func TestCoreAccessReaderFailsWhenCoreAnswersSomethingElse(t *testing.T) {
	api, requester := toolsettest.NewFakeAPIRequester(t)
	api.RespondWithFixture(t, http.MethodGet, "/api/core/v1/capabilities", http.StatusOK, "capabilities_core_only.json")
	api.Handle(http.MethodGet, "/api/core/v1/me", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("not json")) })
	reader := core.NewCoreAccessReader(requester)

	if _, err := reader.ReadAccess(context.Background()); err == nil {
		t.Fatal("ReadAccess accepted an undecodable /me")
	}
	api.RespondEmpty(http.MethodGet, "/api/core/v1/capabilities", http.StatusServiceUnavailable)
	if _, err := reader.ReadAccess(context.Background()); err == nil {
		t.Fatal("ReadAccess ignored a failing /capabilities")
	}
}
