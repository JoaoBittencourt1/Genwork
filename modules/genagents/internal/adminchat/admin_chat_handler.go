package adminchat

import (
	"net/http"
	"strconv"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/access"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runs"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

type ChatSummaryResponse struct {
	Token     string    `json:"token"`
	Title     string    `json:"title"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ChatListResponse struct {
	Chats []ChatSummaryResponse `json:"chats"`
}

type ChatResponse struct {
	ChatSummaryResponse
	BotName      string                  `json:"botName"`
	Entries      []runs.EntryResponse    `json:"entries"`
	LastSequence int64                   `json:"lastSequence"`
	ActiveRun    *runs.ActiveRunResponse `json:"activeRun"`
}

func newChatSummary(conversation runs.Conversation) ChatSummaryResponse {
	title := ""
	if conversation.Title != nil {
		title = *conversation.Title
	}
	return ChatSummaryResponse{Token: conversation.Token, Title: title, UpdatedAt: conversation.UpdatedAt}
}

func NewChatResponse(view ChatView) ChatResponse {
	response := ChatResponse{
		ChatSummaryResponse: newChatSummary(view.Conversation), BotName: view.BotName, Entries: runs.NewEntryResponses(view.Entries), LastSequence: view.LastSequence,
	}
	if view.ActiveRun != nil {
		response.ActiveRun = &runs.ActiveRunResponse{Token: view.ActiveRun.Token, Kind: view.ActiveRun.Kind, State: view.ActiveRun.State, BotName: view.BotName}
	}
	return response
}

type StartChatRequest struct {
	BotToken string `json:"botToken"`
	Title    string `json:"title"`
}

type RenameChatRequest struct {
	Title string `json:"title"`
}

type ChatMessageRequest struct {
	Text string `json:"text"`
}

type AdminChatHandler struct {
	service *AdminChatService
}

func NewAdminChatHandler(service *AdminChatService) *AdminChatHandler {
	return &AdminChatHandler{service: service}
}

// Register: every route needs genagents.adminchat.use, and every chat route also checks the chat is the caller's.
func (handler *AdminChatHandler) Register(router *httpapi.Router) {
	use := httpapi.RequirePermission(access.PermissionAdminChatUse)
	router.HandleFunc("GET /api/genagents/v1/admin-chats", use, handler.listChats)
	router.HandleFunc("POST /api/genagents/v1/admin-chats", use, handler.startChat)
	router.HandleFunc("GET /api/genagents/v1/admin-chats/{token}", use, handler.readChat)
	router.HandleFunc("PATCH /api/genagents/v1/admin-chats/{token}", use, handler.renameChat)
	router.HandleFunc("DELETE /api/genagents/v1/admin-chats/{token}", use, handler.deleteChat)
	router.HandleFunc("POST /api/genagents/v1/admin-chats/{token}/messages", use, handler.postMessage)
	router.HandleFunc("POST /api/genagents/v1/admin-chats/{token}/stop", use, handler.stopChat)
	router.HandleFunc("POST /api/genagents/v1/admin-chats/{token}/reopen", use, handler.reopenChat)
}

func callerFrom(r *http.Request) Caller {
	principal, _ := httpapi.PrincipalFrom(r.Context())
	return Caller{UserToken: principal.UserToken, AccessToken: principal.AccessToken}
}

func (handler *AdminChatHandler) listChats(w http.ResponseWriter, r *http.Request) {
	listed, err := handler.service.ListChats(r.Context(), callerFrom(r))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	response := ChatListResponse{Chats: make([]ChatSummaryResponse, 0, len(listed))}
	for _, conversation := range listed {
		response.Chats = append(response.Chats, newChatSummary(conversation))
	}
	httpapi.WriteJSON(w, http.StatusOK, response)
}

func (handler *AdminChatHandler) startChat(w http.ResponseWriter, r *http.Request) {
	var request StartChatRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	conversation, err := handler.service.StartChat(r.Context(), callerFrom(r), request.BotToken, request.Title)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, newChatSummary(conversation))
}

func (handler *AdminChatHandler) readChat(w http.ResponseWriter, r *http.Request) {
	var after int64
	if raw := r.URL.Query().Get("after"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 0 {
			httpapi.WriteError(w, r, httpapi.NewValidationError(map[string]string{"after": "invalid"}))
			return
		}
		after = parsed
	}
	view, err := handler.service.ReadChat(r.Context(), callerFrom(r), r.PathValue("token"), after)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, NewChatResponse(view))
}

func (handler *AdminChatHandler) renameChat(w http.ResponseWriter, r *http.Request) {
	var request RenameChatRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	if err := handler.service.RenameChat(r.Context(), callerFrom(r), r.PathValue("token"), request.Title); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler *AdminChatHandler) deleteChat(w http.ResponseWriter, r *http.Request) {
	if err := handler.service.DeleteChat(r.Context(), callerFrom(r), r.PathValue("token")); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler *AdminChatHandler) postMessage(w http.ResponseWriter, r *http.Request) {
	var request ChatMessageRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	if err := handler.service.PostMessage(r.Context(), callerFrom(r), r.PathValue("token"), request.Text); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (handler *AdminChatHandler) stopChat(w http.ResponseWriter, r *http.Request) {
	if err := handler.service.StopChat(r.Context(), callerFrom(r), r.PathValue("token")); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler *AdminChatHandler) reopenChat(w http.ResponseWriter, r *http.Request) {
	if err := handler.service.ReopenChat(r.Context(), callerFrom(r), r.PathValue("token")); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
