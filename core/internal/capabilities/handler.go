package capabilities

import (
	"context"
	"net/http"
	"slices"

	"github.com/JoaoBittencourt1/Genwork/core/internal/modules"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
)

type EnabledModuleLister interface {
	ListEnabled(ctx context.Context) ([]modules.Module, error)
}

type RestrictedModuleFinder interface {
	FindRestrictedModuleNames(ctx context.Context, userID int64) ([]string, error)
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
	modules      EnabledModuleLister
	restrictions RestrictedModuleFinder
	core         CoreCapabilityResponse
}

func NewCapabilitiesHandler(enabledModules EnabledModuleLister, restrictions RestrictedModuleFinder, coreVersion, coreAPIVersion string) *CapabilitiesHandler {
	return &CapabilitiesHandler{
		modules: enabledModules, restrictions: restrictions, core: CoreCapabilityResponse{Version: coreVersion, API: coreAPIVersion},
	}
}

func (handler *CapabilitiesHandler) Register(router *platform.Router) {
	router.HandleFunc("GET /api/core/v1/capabilities", platform.Authenticated, handler.listCapabilities)
}

func (handler *CapabilitiesHandler) listCapabilities(w http.ResponseWriter, r *http.Request) {
	principal, _ := platform.PrincipalFrom(r.Context())
	enabled, err := handler.modules.ListEnabled(r.Context())
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	restricted, err := handler.restrictions.FindRestrictedModuleNames(r.Context(), principal.UserID)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	response := CapabilitiesResponse{Core: handler.core, Modules: make([]ModuleCapabilityResponse, 0, len(enabled))}
	for _, module := range enabled {
		if slices.Contains(restricted, module.Name) {
			continue
		}
		response.Modules = append(response.Modules, ModuleCapabilityResponse{
			ID: module.Name, Version: module.Version, APIPrefix: module.APIPrefix,
		})
	}
	platform.WriteJSON(w, http.StatusOK, response)
}
