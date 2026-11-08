package runexec

import (
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/access"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runners"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

type RunnerResponse struct {
	Token                 string            `json:"token"`
	Name                  string            `json:"name"`
	Online                bool              `json:"online"`
	Revoked               bool              `json:"revoked"`
	Registered            bool              `json:"registered"`
	LastHeartbeatAt       *time.Time        `json:"lastHeartbeatAt"`
	RegistrationExpiresAt *time.Time        `json:"registrationExpiresAt"`
	ContainerEngine       *string           `json:"containerEngine"`
	SandboxImage          *string           `json:"sandboxImage"`
	CustomImage           bool              `json:"customImage"`
	DirectoryMode         bool              `json:"directoryMode"`
	HasGithubCredentials  bool              `json:"hasGithubCredentials"`
	MaxConcurrency        int               `json:"maxConcurrency"`
	Tools                 map[string]string `json:"tools"`
	CleanupPending        bool              `json:"cleanupPending"`
	LastCleanupAt         *time.Time        `json:"lastCleanupAt"`
	LastCleanupFreedBytes *int64            `json:"lastCleanupFreedBytes"`
}

type RunnerListResponse struct {
	Runners []RunnerResponse `json:"runners"`
}

type CreatedRunnerResponse struct {
	RunnerResponse
	RegistrationToken string `json:"registrationToken"`
}

func NewRunnerResponse(runner runners.Runner, now time.Time) RunnerResponse {
	tools := runner.ReportedTools
	if tools == nil {
		tools = map[string]string{}
	}
	return RunnerResponse{
		Token: runner.Token, Name: runner.Name, Online: runner.IsOnline(now), Revoked: runner.IsRevoked(), Registered: runner.HasSecret,
		LastHeartbeatAt: runner.LastHeartbeatAt, RegistrationExpiresAt: runner.RegistrationExpiresAt,
		ContainerEngine: runner.ContainerEngine, SandboxImage: runner.SandboxImage,
		CustomImage: runner.CustomImage, DirectoryMode: runner.DirectoryMode, HasGithubCredentials: runner.HasGithubCredentials,
		MaxConcurrency: runner.MaxConcurrency, Tools: tools,
		CleanupPending: runner.CleanupPending(), LastCleanupAt: runner.LastCleanupAt, LastCleanupFreedBytes: runner.LastCleanupFreedBytes,
	}
}

type CreateRunnerRequest struct {
	Name string `json:"name"`
}

type RunnerHandler struct {
	service *RunnerService
	now     func() time.Time
}

func NewRunnerHandler(service *RunnerService, now func() time.Time) *RunnerHandler {
	return &RunnerHandler{service: service, now: now}
}

func (handler *RunnerHandler) Register(router *httpapi.Router) {
	manage := httpapi.RequirePermission(access.PermissionRunnerManage)
	router.HandleFunc("GET /api/genagents/v1/runners", manage, handler.listRunners)
	router.HandleFunc("POST /api/genagents/v1/runners", manage, handler.createRunner)
	router.HandleFunc("DELETE /api/genagents/v1/runners/{token}", manage, handler.revokeRunner)
	router.HandleFunc("POST /api/genagents/v1/runners/{token}/removal", manage, handler.removeRunner)
}

func (handler *RunnerHandler) listRunners(w http.ResponseWriter, r *http.Request) {
	listed, err := handler.service.ListRunners(r.Context())
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	response := RunnerListResponse{Runners: make([]RunnerResponse, 0, len(listed))}
	for _, runner := range listed {
		response.Runners = append(response.Runners, NewRunnerResponse(runner, handler.now()))
	}
	httpapi.WriteJSON(w, http.StatusOK, response)
}

func (handler *RunnerHandler) createRunner(w http.ResponseWriter, r *http.Request) {
	var request CreateRunnerRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	runner, registrationToken, err := handler.service.CreateRunner(r.Context(), request.Name)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpapi.WriteJSON(w, http.StatusCreated, CreatedRunnerResponse{
		RunnerResponse: NewRunnerResponse(runner, handler.now()), RegistrationToken: registrationToken,
	})
}

func (handler *RunnerHandler) revokeRunner(w http.ResponseWriter, r *http.Request) {
	if err := handler.service.RevokeRunner(r.Context(), r.PathValue("token")); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler *RunnerHandler) removeRunner(w http.ResponseWriter, r *http.Request) {
	if err := handler.service.RemoveRunner(r.Context(), r.PathValue("token")); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
