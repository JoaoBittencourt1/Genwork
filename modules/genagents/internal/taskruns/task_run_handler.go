package taskruns

import (
	"net/http"
	"strconv"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/access"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runs"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

type TaskConversationResponse struct {
	Conversation *string                 `json:"conversation"`
	Entries      []runs.EntryResponse    `json:"entries"`
	LastSequence int64                   `json:"lastSequence"`
	ActiveRun    *runs.ActiveRunResponse `json:"activeRun"`
}

func NewTaskConversationResponse(view TaskConversationView) TaskConversationResponse {
	response := TaskConversationResponse{Entries: runs.NewEntryResponses(view.Entries), LastSequence: view.LastSequence}
	if view.Conversation != nil {
		response.Conversation = &view.Conversation.Token
	}
	if view.ActiveRun != nil {
		response.ActiveRun = &runs.ActiveRunResponse{Token: view.ActiveRun.Run.Token, Kind: view.ActiveRun.Run.Kind, State: view.ActiveRun.Run.State, BotName: view.ActiveRun.BotName}
	}
	return response
}

type RunSummaryResponse struct {
	Token         string              `json:"token"`
	Kind          runs.RunKind        `json:"kind"`
	State         runs.RunState       `json:"state"`
	FailureReason *runs.FailureReason `json:"failureReason"`
	BotName       string              `json:"botName"`
	TaskToken     *string             `json:"taskToken"`
	TaskKey       string              `json:"taskKey"`
	CreatedAt     time.Time           `json:"createdAt"`
	EndedAt       *time.Time          `json:"endedAt"`
}

type RunListResponse struct {
	Runs []RunSummaryResponse `json:"runs"`
}

type TaskAssignmentRequest struct {
	TaskToken string `json:"taskToken"`
}

type BotReviewRequest struct {
	BotToken string `json:"botToken"`
}

type BotReviewResponse struct {
	RunToken string        `json:"runToken"`
	State    runs.RunState `json:"state"`
}

type PostMessageRequest struct {
	Text string `json:"text"`
}

type TaskRunHandler struct {
	runs          *TaskRunService
	conversations *TaskConversationService
}

func NewTaskRunHandler(runService *TaskRunService, conversations *TaskConversationService) *TaskRunHandler {
	return &TaskRunHandler{runs: runService, conversations: conversations}
}

func (handler *TaskRunHandler) Register(router *httpapi.Router) {
	read := httpapi.RequirePermission(access.PermissionRunRead)
	write := httpapi.RequirePermission(access.PermissionRunWrite)
	router.HandleFunc("GET /api/genagents/v1/runs", read, handler.listRuns)
	router.HandleFunc("POST /api/genagents/v1/task-assignments", write, handler.notifyAssignment)
	router.HandleFunc("POST /api/genagents/v1/tasks/{taskToken}/bot-reviews", write, handler.requestReview)
	router.HandleFunc("GET /api/genagents/v1/tasks/{taskToken}/conversation", read, handler.readConversation)
	router.HandleFunc("POST /api/genagents/v1/tasks/{taskToken}/conversation/messages", write, handler.postMessage)
	router.HandleFunc("POST /api/genagents/v1/tasks/{taskToken}/conversation/stop", write, handler.stopRuns)
}

func callerFrom(r *http.Request) Caller {
	principal, _ := httpapi.PrincipalFrom(r.Context())
	return Caller{UserToken: principal.UserToken, AccessToken: principal.AccessToken}
}

func (handler *TaskRunHandler) notifyAssignment(w http.ResponseWriter, r *http.Request) {
	var request TaskAssignmentRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	if request.TaskToken == "" {
		httpapi.WriteError(w, r, httpapi.NewValidationError(map[string]string{"taskToken": "required"}))
		return
	}
	if err := handler.runs.HandleAssignment(r.Context(), callerFrom(r), request.TaskToken); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (handler *TaskRunHandler) requestReview(w http.ResponseWriter, r *http.Request) {
	var request BotReviewRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	run, err := handler.runs.RequestReview(r.Context(), callerFrom(r), r.PathValue("taskToken"), request.BotToken)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusAccepted, BotReviewResponse{RunToken: run.Token, State: run.State})
}

func (handler *TaskRunHandler) readConversation(w http.ResponseWriter, r *http.Request) {
	var after int64
	if raw := r.URL.Query().Get("after"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 0 {
			httpapi.WriteError(w, r, httpapi.NewValidationError(map[string]string{"after": "invalid"}))
			return
		}
		after = parsed
	}
	view, err := handler.conversations.ReadConversation(r.Context(), callerFrom(r), r.PathValue("taskToken"), after)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, NewTaskConversationResponse(view))
}

func (handler *TaskRunHandler) postMessage(w http.ResponseWriter, r *http.Request) {
	var request PostMessageRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	if err := handler.conversations.PostMessage(r.Context(), callerFrom(r), r.PathValue("taskToken"), request.Text); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (handler *TaskRunHandler) stopRuns(w http.ResponseWriter, r *http.Request) {
	if err := handler.conversations.StopRuns(r.Context(), callerFrom(r), r.PathValue("taskToken")); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

const (
	defaultRunListLimit = 50
	maxRunListLimit     = 200
)

func (handler *TaskRunHandler) listRuns(w http.ResponseWriter, r *http.Request) {
	limit := defaultRunListLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > maxRunListLimit {
			httpapi.WriteError(w, r, httpapi.NewValidationError(map[string]string{"limit": "invalid"}))
			return
		}
		limit = parsed
	}
	listed, err := handler.runs.ListRuns(r.Context(), r.URL.Query().Get("state") == "active", limit)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	response := RunListResponse{Runs: make([]RunSummaryResponse, 0, len(listed))}
	for _, summary := range listed {
		response.Runs = append(response.Runs, RunSummaryResponse{
			Token: summary.Run.Token, Kind: summary.Run.Kind, State: summary.Run.State, FailureReason: summary.Run.FailureReason, BotName: summary.BotName,
			TaskToken: summary.TaskToken, TaskKey: summary.Run.Spec.TaskKey, CreatedAt: summary.Run.CreatedAt, EndedAt: summary.Run.EndedAt,
		})
	}
	httpapi.WriteJSON(w, http.StatusOK, response)
}
