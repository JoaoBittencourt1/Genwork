package storagecleanup

import (
	"net/http"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/access"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

type CleanupResponse struct {
	RemovedSignIns        int `json:"removedSignIns"`
	RemovedAuthorizations int `json:"removedAuthorizations"`
	RunnersAsked          int `json:"runnersAsked"`
}

type StorageCleanupHandler struct {
	service *StorageCleanupService
}

func NewStorageCleanupHandler(service *StorageCleanupService) *StorageCleanupHandler {
	return &StorageCleanupHandler{service: service}
}

func (handler *StorageCleanupHandler) Register(router *httpapi.Router) {
	router.HandleFunc("POST /api/genagents/v1/runners/cleanup", httpapi.RequirePermission(access.PermissionRunnerManage), handler.clean)
}

func (handler *StorageCleanupHandler) clean(w http.ResponseWriter, r *http.Request) {
	result, err := handler.service.Clean(r.Context())
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, CleanupResponse{
		RemovedSignIns: result.RemovedSignIns, RemovedAuthorizations: result.RemovedAuthorizations, RunnersAsked: result.RunnersAsked,
	})
}
