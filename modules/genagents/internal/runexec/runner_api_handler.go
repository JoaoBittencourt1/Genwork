package runexec

import (
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runners"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runs"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

const (
	claimLongPollLimit = 25 * time.Second
	claimPollInterval  = time.Second
)

// RunnerAPIHandler serves /api/genagents/v1/runner/...: the routes a runner process calls, each authenticated with the
// runner secret (runners.RequireRunner).
type RunnerAPIHandler struct {
	runners       *RunnerService
	claims        *RunClaimService
	events        *RunEventService
	tokens        *GenworkTokenService
	github        *GithubTokenService
	policy        RunAccessPolicy
	runRepository runs.RunRepository
	entries       runs.EntryRepository
	db            runDatabase
	genworkMCPURL string
	now           func() time.Time
	pollInterval  time.Duration
	pollLimit     time.Duration
}

// runDatabase is the pool the handler reads messages through; it is the same pool every service uses.
type runDatabase interface {
	platform.DBTX
}

func NewRunnerAPIHandler(db runDatabase, runnerService *RunnerService, claims *RunClaimService, events *RunEventService, tokens *GenworkTokenService, github *GithubTokenService, genworkMCPURL string, now func() time.Time) *RunnerAPIHandler {
	return &RunnerAPIHandler{
		runners: runnerService, claims: claims, events: events, tokens: tokens, github: github, db: db, genworkMCPURL: genworkMCPURL, now: now,
		pollInterval: claimPollInterval, pollLimit: claimLongPollLimit,
	}
}

// WithLongPoll changes how long a claim waits for work and how often it looks, for tests and tuning.
func (handler *RunnerAPIHandler) WithLongPoll(limit, interval time.Duration) *RunnerAPIHandler {
	handler.pollLimit, handler.pollInterval = limit, interval
	return handler
}

func (handler *RunnerAPIHandler) Register(router *httpapi.Router) {
	router.HandleFunc("POST /api/genagents/v1/runner/register", httpapi.Public, handler.register)
	router.HandleFunc("POST /api/genagents/v1/runner/heartbeat", httpapi.Public, handler.authenticated(handler.heartbeat))
	router.HandleFunc("POST /api/genagents/v1/runner/accounts/{token}/status", httpapi.Public, handler.authenticated(handler.reportAccountStatus))
	router.HandleFunc("POST /api/genagents/v1/runner/claims", httpapi.Public, handler.authenticated(handler.claim))
	router.HandleFunc("GET /api/genagents/v1/runner/runs/{token}", httpapi.Public, handler.authenticated(handler.findRun))
	router.HandleFunc("POST /api/genagents/v1/runner/runs/{token}/events", httpapi.Public, handler.authenticated(handler.postEvents))
	router.HandleFunc("GET /api/genagents/v1/runner/runs/{token}/messages", httpapi.Public, handler.authenticated(handler.listMessages))
	router.HandleFunc("POST /api/genagents/v1/runner/runs/{token}/messages/ack", httpapi.Public, handler.authenticated(handler.acknowledgeMessages))
	router.HandleFunc("POST /api/genagents/v1/runner/runs/{token}/genwork-token", httpapi.Public, handler.authenticated(handler.issueGenworkToken))
	router.HandleFunc("POST /api/genagents/v1/runner/runs/{token}/github-token", httpapi.Public, handler.authenticated(handler.issueGithubToken))
}

func (handler *RunnerAPIHandler) authenticated(next runners.RunnerRequestHandler) http.HandlerFunc {
	return runners.RequireRunner(handler.runners, next)
}

type RegisterRunnerRequest struct {
	RegistrationToken string `json:"registrationToken"`
}

type RegisteredRunnerResponse struct {
	RunnerToken string `json:"runnerToken"`
	Name        string `json:"name"`
	Secret      string `json:"secret"`
}

func (handler *RunnerAPIHandler) register(w http.ResponseWriter, r *http.Request) {
	var request RegisterRunnerRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	runner, secret, err := handler.runners.ExchangeRegistration(r.Context(), request.RegistrationToken)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpapi.WriteJSON(w, http.StatusOK, RegisteredRunnerResponse{RunnerToken: runner.Token, Name: runner.Name, Secret: secret})
}

type HeartbeatRequest struct {
	ContainerEngine      string            `json:"containerEngine"`
	SandboxImage         string            `json:"sandboxImage"`
	CustomImage          bool              `json:"customImage"`
	DirectoryMode        bool              `json:"directoryMode"`
	Tools                map[string]string `json:"tools"`
	HasGithubCredentials bool              `json:"hasGithubCredentials"`
	MaxConcurrency       int               `json:"maxConcurrency"`
	// Cleanup is set on the one heartbeat after the runner finished a requested cleanup.
	Cleanup *CleanupReportRequest `json:"cleanup"`
}

type CleanupReportRequest struct {
	FreedBytes int64 `json:"freedBytes"`
}

type AccountAssignmentResponse struct {
	Token    string `json:"token"`
	Provider string `json:"provider"`
}

type HeartbeatResponse struct {
	Accounts []AccountAssignmentResponse `json:"accounts"`
	// CleanupRequested asks the runner to free space: an admin asked and the runner has not reported doing it since.
	CleanupRequested bool `json:"cleanupRequested"`
}

func (handler *RunnerAPIHandler) heartbeat(w http.ResponseWriter, r *http.Request, runner runners.Runner) {
	var request HeartbeatRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	heartbeat := runners.Heartbeat{
		ContainerEngine: request.ContainerEngine, SandboxImage: request.SandboxImage, CustomImage: request.CustomImage,
		DirectoryMode: request.DirectoryMode, Tools: request.Tools, HasGithubCredentials: request.HasGithubCredentials, MaxConcurrency: request.MaxConcurrency,
	}
	if request.Cleanup != nil {
		heartbeat.CleanupFreedBytes = &request.Cleanup.FreedBytes
	}
	answer, err := handler.runners.RecordHeartbeat(r.Context(), runner, heartbeat)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	response := HeartbeatResponse{Accounts: make([]AccountAssignmentResponse, 0, len(answer.Assignments)), CleanupRequested: answer.CleanupRequested}
	for _, assignment := range answer.Assignments {
		response.Accounts = append(response.Accounts, AccountAssignmentResponse{Token: assignment.Token, Provider: string(assignment.Provider)})
	}
	httpapi.WriteJSON(w, http.StatusOK, response)
}

type AccountStatusRequest struct {
	SignedIn bool   `json:"signedIn"`
	Message  string `json:"message"`
}

func (handler *RunnerAPIHandler) reportAccountStatus(w http.ResponseWriter, r *http.Request, runner runners.Runner) {
	var request AccountStatusRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	if err := handler.runners.ReportAccountStatus(r.Context(), runner, r.PathValue("token"), request.SignedIn, request.Message); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type ClaimRequest struct {
	FreeSlots int `json:"freeSlots"`
}

// claim long-polls: it answers as soon as a run is claimed, and with 204 when none came within the limit.
func (handler *RunnerAPIHandler) claim(w http.ResponseWriter, r *http.Request, runner runners.Runner) {
	var request ClaimRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	deadline := time.NewTimer(handler.pollLimit)
	defer deadline.Stop()
	for {
		claimed, err := handler.claims.ClaimRun(r.Context(), runner, request.FreeSlots)
		if err != nil {
			httpapi.WriteError(w, r, err)
			return
		}
		if claimed != nil {
			w.Header().Set("Cache-Control", "no-store")
			httpapi.WriteJSON(w, http.StatusOK, NewClaimedRunResponse(*claimed, handler.genworkMCPURL))
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-deadline.C:
			w.WriteHeader(http.StatusNoContent)
			return
		case <-time.After(handler.pollInterval):
		}
	}
}

type RunStateResponse struct {
	Token string        `json:"token"`
	State runs.RunState `json:"state"`
}

func (handler *RunnerAPIHandler) findRun(w http.ResponseWriter, r *http.Request, runner runners.Runner) {
	run, err := handler.policy.FindRunOfRunner(r.Context(), handler.db, runner, r.PathValue("token"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, RunStateResponse{Token: run.Token, State: run.State})
}

type PostEventsRequest struct {
	Events []EventInput `json:"events"`
}

type PostEventsResponse struct {
	Accepted int           `json:"accepted"`
	State    runs.RunState `json:"state"`
}

func (handler *RunnerAPIHandler) postEvents(w http.ResponseWriter, r *http.Request, runner runners.Runner) {
	var request PostEventsRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	result, err := handler.events.Ingest(r.Context(), runner, r.PathValue("token"), request.Events)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, PostEventsResponse{Accepted: result.Accepted, State: result.State})
}

type MessageResponse struct {
	Sequence    int64   `json:"sequence"`
	Text        string  `json:"text"`
	AuthorToken *string `json:"authorToken"`
}

type MessageListResponse struct {
	Messages []MessageResponse `json:"messages"`
}

func (handler *RunnerAPIHandler) listMessages(w http.ResponseWriter, r *http.Request, runner runners.Runner) {
	run, err := handler.policy.FindRunOfRunner(r.Context(), handler.db, runner, r.PathValue("token"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	pending, err := handler.entries.ListUndeliveredMessages(r.Context(), handler.db, run.ConversationID)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	response := MessageListResponse{Messages: make([]MessageResponse, 0, len(pending))}
	for _, entry := range pending {
		response.Messages = append(response.Messages, MessageResponse{Sequence: entry.Sequence, Text: runs.MessageText(entry.Payload), AuthorToken: entry.AuthorToken})
	}
	httpapi.WriteJSON(w, http.StatusOK, response)
}

type AcknowledgeMessagesRequest struct {
	UpToSequence int64 `json:"upToSequence"`
}

func (handler *RunnerAPIHandler) acknowledgeMessages(w http.ResponseWriter, r *http.Request, runner runners.Runner) {
	var request AcknowledgeMessagesRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	run, err := handler.policy.FindRunOfRunner(r.Context(), handler.db, runner, r.PathValue("token"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	if err := handler.entries.MarkDelivered(r.Context(), handler.db, run.ConversationID, request.UpToSequence, handler.now()); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type GenworkTokenResponse struct {
	AccessToken string    `json:"accessToken"`
	ExpiresAt   time.Time `json:"expiresAt"`
	MCPURL      string    `json:"mcpUrl"`
}

func (handler *RunnerAPIHandler) issueGenworkToken(w http.ResponseWriter, r *http.Request, runner runners.Runner) {
	token, err := handler.tokens.IssueToken(r.Context(), runner, r.PathValue("token"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpapi.WriteJSON(w, http.StatusOK, GenworkTokenResponse{AccessToken: token.AccessToken, ExpiresAt: token.ExpiresAt, MCPURL: handler.genworkMCPURL})
}

type GithubTokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func (handler *RunnerAPIHandler) issueGithubToken(w http.ResponseWriter, r *http.Request, runner runners.Runner) {
	token, err := handler.github.IssueToken(r.Context(), runner, r.PathValue("token"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpapi.WriteJSON(w, http.StatusOK, GithubTokenResponse{Token: token.Token, ExpiresAt: token.ExpiresAt})
}
