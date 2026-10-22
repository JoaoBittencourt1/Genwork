package githubsync

import (
	"net/http"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/access"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

// BoardGithubSettingsResponse uses null for "do not move".
type BoardGithubSettingsResponse struct {
	OpenedColumnToken          *string  `json:"openedColumnToken"`
	ReviewRequestedColumnToken *string  `json:"reviewRequestedColumnToken"`
	MergedColumnToken          *string  `json:"mergedColumnToken"`
	FinalBranches              []string `json:"finalBranches"`
}

func NewBoardGithubSettingsResponse(settings BoardGithubSettings) BoardGithubSettingsResponse {
	return BoardGithubSettingsResponse{
		OpenedColumnToken:          mappedColumnToken(settings.Opened),
		ReviewRequestedColumnToken: mappedColumnToken(settings.ReviewRequested),
		MergedColumnToken:          mappedColumnToken(settings.Merged),
		FinalBranches:              settings.FinalBranches,
	}
}

func mappedColumnToken(column *MappedColumn) *string {
	if column == nil {
		return nil
	}
	return &column.Token
}

type BoardGithubSettingsHandler struct {
	service *BoardGithubSettingsService
}

func NewBoardGithubSettingsHandler(service *BoardGithubSettingsService) *BoardGithubSettingsHandler {
	return &BoardGithubSettingsHandler{service: service}
}

func (handler *BoardGithubSettingsHandler) Register(router *httpapi.Router) {
	router.HandleFunc("GET /api/gentasks/v1/boards/{token}/github-settings", httpapi.RequirePermission(access.PermissionBoardRead), handler.findSettings)
	router.HandleFunc("PATCH /api/gentasks/v1/boards/{token}/github-settings", httpapi.RequirePermission(access.PermissionBoardManage), handler.updateSettings)
}

func (handler *BoardGithubSettingsHandler) findSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := handler.service.FindSettings(r.Context(), r.PathValue("token"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, NewBoardGithubSettingsResponse(settings))
}

func (handler *BoardGithubSettingsHandler) updateSettings(w http.ResponseWriter, r *http.Request) {
	var request UpdateBoardGithubSettingsRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	changes, err := request.Validate()
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	settings, err := handler.service.UpdateSettings(r.Context(), r.PathValue("token"), changes)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, NewBoardGithubSettingsResponse(settings))
}
