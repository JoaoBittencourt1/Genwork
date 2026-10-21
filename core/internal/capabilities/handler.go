package capabilities

import (
	"context"
	"net/http"

	"github.com/JoaoBittencourt1/Genwork/core/internal/modules"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
)

type EnabledModuleLister interface {
	ListEnabled(ctx context.Context) ([]modules.Module, error)
}

type CoreCapabilityResponse struct {
	Version string `json:"version"`
	API     string `json:"api"`
}

type ModuleCapabilityResponse struct {
	ID        string `json:"id"`
	Version   string `json:"version"`
	APIPrefix string `json:"apiPrefix"`
}

type CapabilitiesResponse struct {
	Core    CoreCapabilityResponse     `json:"core"`
	Modules []ModuleCapabilityResponse `json:"modules"`
}

type CapabilitiesHandler struct {
	modules EnabledModuleLister
	core    CoreCapabilityResponse
}

func NewCapabilitiesHandler(enabledModules EnabledModuleLister, coreVersion, coreAPIVersion string) *CapabilitiesHandler {
	return &CapabilitiesHandler{modules: enabledModules, core: CoreCapabilityResponse{Version: coreVersion, API: coreAPIVersion}}
}

func (handler *CapabilitiesHandler) Register(router *platform.Router) {
	router.HandleFunc("GET /api/core/v1/capabilities", platform.Authenticated, handler.listCapabilities)
}

func (handler *CapabilitiesHandler) listCapabilities(w http.ResponseWriter, r *http.Request) {
	enabled, err := handler.modules.ListEnabled(r.Context())
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	response := CapabilitiesResponse{Core: handler.core, Modules: make([]ModuleCapabilityResponse, 0, len(enabled))}
	for _, module := range enabled {
		response.Modules = append(response.Modules, ModuleCapabilityResponse{
			ID: module.Name, Version: module.Version, APIPrefix: module.APIPrefix,
		})
	}
	platform.WriteJSON(w, http.StatusOK, response)
}
