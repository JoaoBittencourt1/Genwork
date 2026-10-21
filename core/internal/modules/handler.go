package modules

import (
	"net/http"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
)

type RegisteredModuleResponse struct {
	Token   string `json:"token"`
	ID      string `json:"id"`
	Version string `json:"version"`
	Enabled bool   `json:"enabled"`
}

type ModuleHandler struct {
	service *ModuleService
}

func NewModuleHandler(service *ModuleService) *ModuleHandler {
	return &ModuleHandler{service: service}
}

func (handler *ModuleHandler) Register(router *platform.Router) {
	router.HandleFunc("PUT /internal/v1/modules/{id}", platform.ModuleCredential, handler.registerModule)
}

func (handler *ModuleHandler) registerModule(w http.ResponseWriter, r *http.Request) {
	var request RegisterModuleRequest
	if err := platform.DecodeJSON(w, r, &request); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	registration, err := request.ToRegistration(r.PathValue("id"))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	module, err := handler.service.Register(r.Context(), registration)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WriteJSON(w, http.StatusOK, RegisteredModuleResponse{
		Token: module.Token, ID: module.Name, Version: module.Version, Enabled: module.Enabled,
	})
}
