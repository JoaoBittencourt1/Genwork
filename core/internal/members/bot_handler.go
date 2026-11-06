package members

import (
	"net/http"

	"github.com/JoaoBittencourt1/Genwork/core/internal/access"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
)

type BotMemberHandler struct {
	service *BotMemberService
}

func NewBotMemberHandler(service *BotMemberService) *BotMemberHandler {
	return &BotMemberHandler{service: service}
}

func (handler *BotMemberHandler) Register(router *platform.Router) {
	manage := platform.RequirePermission(access.PermissionMemberManage)
	router.HandleFunc("POST /api/core/v1/bots", manage, handler.createBot)
	router.HandleFunc("PATCH /api/core/v1/bots/{token}", manage, handler.updateBot)
	router.HandleFunc("POST /api/core/v1/bots/{token}/secret", manage, handler.rotateSecret)
	router.HandleFunc("DELETE /api/core/v1/bots/{token}", manage, handler.removeBot)
}

func (handler *BotMemberHandler) createBot(w http.ResponseWriter, r *http.Request) {
	var request CreateBotRequest
	if err := platform.DecodeJSON(w, r, &request); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	principal, _ := platform.PrincipalFrom(r.Context())
	member, secret, err := handler.service.Create(r.Context(), principal, request)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	writeSecretJSON(w, http.StatusCreated, NewBotSecretResponse(member, secret))
}

func (handler *BotMemberHandler) updateBot(w http.ResponseWriter, r *http.Request) {
	var request UpdateBotRequest
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

func (handler *BotMemberHandler) rotateSecret(w http.ResponseWriter, r *http.Request) {
	principal, _ := platform.PrincipalFrom(r.Context())
	member, secret, err := handler.service.RotateSecret(r.Context(), principal, r.PathValue("token"))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	writeSecretJSON(w, http.StatusOK, NewBotSecretResponse(member, secret))
}

func (handler *BotMemberHandler) removeBot(w http.ResponseWriter, r *http.Request) {
	principal, _ := platform.PrincipalFrom(r.Context())
	if err := handler.service.Remove(r.Context(), principal, r.PathValue("token")); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeSecretJSON keeps the one-time secret out of any cache.
func writeSecretJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Cache-Control", "no-store")
	platform.WriteJSON(w, status, body)
}
