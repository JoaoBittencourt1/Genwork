package bots

import (
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/access"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/accounts"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

type ReferenceResponse struct {
	Token string `json:"token"`
	Name  string `json:"name"`
}

// BotResponse has no credentials: the member secret and the account's key are never part of it.
type BotResponse struct {
	Token          string              `json:"token"`
	Name           string              `json:"name"`
	Kind           BotKind             `json:"kind"`
	Provider       accounts.Provider   `json:"provider"`
	Model          string              `json:"model"`
	MemberToken    string              `json:"memberToken"`
	Enabled        bool                `json:"enabled"`
	Account        ReferenceResponse   `json:"account"`
	Effort         ReasoningEffort     `json:"effort"`
	MaxTurns       int                 `json:"maxTurns"`
	TimeoutMinutes int                 `json:"timeoutMinutes"`
	Instructions   *string             `json:"instructions"`
	Skills         []ReferenceResponse `json:"skills"`
	MCPServers     []ReferenceResponse `json:"mcpServers"`
	UpdatedAt      time.Time           `json:"updatedAt"`
}

type BotListResponse struct {
	Bots []BotResponse `json:"bots"`
}

func newReferenceResponses(references []Reference) []ReferenceResponse {
	responses := make([]ReferenceResponse, 0, len(references))
	for _, reference := range references {
		responses = append(responses, ReferenceResponse{Token: reference.Token, Name: reference.Name})
	}
	return responses
}

func NewBotResponse(bot Bot) BotResponse {
	return BotResponse{
		Token: bot.Token, Name: bot.Name, Kind: bot.Kind, Provider: bot.Provider, Model: bot.Model, MemberToken: bot.MemberToken,
		Enabled: bot.IsEnabled(), Account: ReferenceResponse{Token: bot.AccountToken, Name: bot.AccountName}, Effort: bot.Effort,
		MaxTurns: bot.MaxTurns, TimeoutMinutes: bot.TimeoutMinutes, Instructions: bot.Instructions,
		Skills: newReferenceResponses(bot.Skills), MCPServers: newReferenceResponses(bot.MCPServers), UpdatedAt: bot.UpdatedAt,
	}
}

type BotHandler struct {
	service *BotService
}

func NewBotHandler(service *BotService) *BotHandler {
	return &BotHandler{service: service}
}

func (handler *BotHandler) Register(router *httpapi.Router) {
	read := httpapi.RequirePermission(access.PermissionBotRead)
	manage := httpapi.RequirePermission(access.PermissionBotManage)
	router.HandleFunc("GET /api/genagents/v1/bots", read, handler.listBots)
	router.HandleFunc("GET /api/genagents/v1/bots/{token}", read, handler.findBot)
	router.HandleFunc("POST /api/genagents/v1/bots", manage, handler.createBot)
	router.HandleFunc("PATCH /api/genagents/v1/bots/{token}", manage, handler.updateBot)
	router.HandleFunc("DELETE /api/genagents/v1/bots/{token}", manage, handler.deleteBot)
	router.HandleFunc("POST /api/genagents/v1/bots/{token}/secret", manage, handler.rotateSecret)
}

func callerFrom(r *http.Request) Caller {
	principal, _ := httpapi.PrincipalFrom(r.Context())
	return Caller{AccessToken: principal.AccessToken}
}

func (handler *BotHandler) listBots(w http.ResponseWriter, r *http.Request) {
	var kind *BotKind
	if raw := r.URL.Query().Get("kind"); raw != "" {
		parsed := BotKind(raw)
		if !parsed.IsValid() {
			httpapi.WriteError(w, r, httpapi.NewValidationError(map[string]string{"kind": "invalid"}))
			return
		}
		kind = &parsed
	}
	listed, err := handler.service.ListBots(r.Context(), kind)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	response := BotListResponse{Bots: make([]BotResponse, 0, len(listed))}
	for _, bot := range listed {
		response.Bots = append(response.Bots, NewBotResponse(bot))
	}
	httpapi.WriteJSON(w, http.StatusOK, response)
}

func (handler *BotHandler) findBot(w http.ResponseWriter, r *http.Request) {
	bot, err := handler.service.FindBot(r.Context(), r.PathValue("token"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, NewBotResponse(bot))
}

func (handler *BotHandler) createBot(w http.ResponseWriter, r *http.Request) {
	var request CreateBotRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	input, err := request.Validate()
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	created, err := handler.service.CreateBot(r.Context(), callerFrom(r), input)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, NewBotResponse(created))
}

func (handler *BotHandler) updateBot(w http.ResponseWriter, r *http.Request) {
	var request UpdateBotRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	changes, err := request.Validate()
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	updated, err := handler.service.UpdateBot(r.Context(), callerFrom(r), r.PathValue("token"), changes)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, NewBotResponse(updated))
}

func (handler *BotHandler) deleteBot(w http.ResponseWriter, r *http.Request) {
	if err := handler.service.DeleteBot(r.Context(), callerFrom(r), r.PathValue("token")); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler *BotHandler) rotateSecret(w http.ResponseWriter, r *http.Request) {
	if err := handler.service.RotateSecret(r.Context(), callerFrom(r), r.PathValue("token")); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
