package updates

import (
	"net/http"

	"github.com/JoaoBittencourt1/Genwork/core/internal/access"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
)

type UpdateHandler struct {
	service *UpdateService
}

func NewUpdateHandler(service *UpdateService) *UpdateHandler {
	return &UpdateHandler{service: service}
}

// Register guards every route with core.system.update, so the updater never hears from anyone else (rule 18).
func (handler *UpdateHandler) Register(router *platform.Router) {
	policy := platform.RequirePermission(access.PermissionSystemUpdate)
	router.HandleFunc("GET /api/core/v1/system/update", policy, handler.findStatus)
	router.HandleFunc("POST /api/core/v1/system/update/check", policy, handler.checkForUpdates)
	router.HandleFunc("POST /api/core/v1/system/update", policy, handler.startUpdate)
}

func (handler *UpdateHandler) findStatus(w http.ResponseWriter, r *http.Request) {
	status, err := handler.service.FindStatus(r.Context())
	writeStatus(w, r, http.StatusOK, status, err)
}

func (handler *UpdateHandler) checkForUpdates(w http.ResponseWriter, r *http.Request) {
	status, err := handler.service.CheckForUpdates(r.Context())
	writeStatus(w, r, http.StatusOK, status, err)
}

func (handler *UpdateHandler) startUpdate(w http.ResponseWriter, r *http.Request) {
	var request StartUpdateRequest
	if err := platform.DecodeJSON(w, r, &request); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	if fields := request.Validate(); fields != nil {
		platform.WriteError(w, r, platform.NewValidationError(fields))
		return
	}
	status, err := handler.service.StartUpdate(r.Context(), request.Version)
	writeStatus(w, r, http.StatusAccepted, status, err)
}

func writeStatus(w http.ResponseWriter, r *http.Request, success int, status UpdaterStatus, err error) {
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WriteJSON(w, success, newUpdateStatusResponse(status))
}
