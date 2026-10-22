package members

import (
	"net/http"

	"github.com/JoaoBittencourt1/Genwork/core/internal/access"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
)

type MemberHandler struct {
	service *MemberService
}

func NewMemberHandler(service *MemberService) *MemberHandler {
	return &MemberHandler{service: service}
}

func (handler *MemberHandler) Register(router *platform.Router) {
	router.HandleFunc("GET /api/core/v1/members", platform.RequirePermission(access.PermissionMemberRead), handler.listMembers)
	router.HandleFunc("PATCH /api/core/v1/members/{token}", platform.RequirePermission(access.PermissionMemberManage), handler.updateMember)
	router.HandleFunc("DELETE /api/core/v1/members/{token}", platform.RequirePermission(access.PermissionMemberManage), handler.removeMember)
}

func (handler *MemberHandler) listMembers(w http.ResponseWriter, r *http.Request) {
	listed, err := handler.service.List(r.Context())
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	response := MemberListResponse{Members: make([]MemberResponse, 0, len(listed))}
	for _, member := range listed {
		response.Members = append(response.Members, NewMemberResponse(member))
	}
	platform.WriteJSON(w, http.StatusOK, response)
}

func (handler *MemberHandler) updateMember(w http.ResponseWriter, r *http.Request) {
	var request UpdateMemberRequest
	if err := platform.DecodeJSON(w, r, &request); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	input, err := request.ToUpdateInput()
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	principal, _ := platform.PrincipalFrom(r.Context())
	member, err := handler.service.Update(r.Context(), principal, r.PathValue("token"), input)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WriteJSON(w, http.StatusOK, NewMemberResponse(member))
}

func (handler *MemberHandler) removeMember(w http.ResponseWriter, r *http.Request) {
	principal, _ := platform.PrincipalFrom(r.Context())
	if err := handler.service.Remove(r.Context(), principal, r.PathValue("token")); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
