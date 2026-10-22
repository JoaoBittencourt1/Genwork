package access

import (
	"net/http"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
)

type RoleResponse struct {
	Token       string   `json:"token"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

// RoleSummaryResponse names a role where its permissions are not needed, such as on an invite or a member.
type RoleSummaryResponse struct {
	Token string `json:"token"`
	Name  string `json:"name"`
}

type RoleListResponse struct {
	Roles []RoleResponse `json:"roles"`
}

func NewRoleResponse(role Role) RoleResponse {
	return RoleResponse{Token: role.Token, Name: role.Name, Permissions: role.Permissions}
}

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (handler *Handler) Register(router *platform.Router) {
	router.HandleFunc("GET /api/core/v1/roles", platform.RequireAnyPermission(PermissionInviteCreate, PermissionMemberManage), handler.listRoles)
}

func (handler *Handler) listRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := handler.service.ListRoles(r.Context())
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	response := RoleListResponse{Roles: make([]RoleResponse, 0, len(roles))}
	for _, role := range roles {
		response.Roles = append(response.Roles, NewRoleResponse(role))
	}
	platform.WriteJSON(w, http.StatusOK, response)
}
