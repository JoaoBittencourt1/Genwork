package accounts

import (
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/access"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

type ModelResponse struct {
	Token       *string `json:"token"`
	ID          string  `json:"id"`
	DisplayName string  `json:"displayName"`
	Custom      bool    `json:"custom"`
}

type ProviderResponse struct {
	Provider Provider        `json:"provider"`
	CLI      string          `json:"cli"`
	Modes    []AccountMode   `json:"modes"`
	Models   []ModelResponse `json:"models"`
}

type ProviderListResponse struct {
	Providers []ProviderResponse `json:"providers"`
}

type AccountRunnerResponse struct {
	Token string `json:"token"`
	Name  string `json:"name"`
}

type AiAccountResponse struct {
	Token       string                 `json:"token"`
	Name        string                 `json:"name"`
	Provider    Provider               `json:"provider"`
	Mode        AccountMode            `json:"mode"`
	Runner      *AccountRunnerResponse `json:"runner"`
	HasAPIKey   bool                   `json:"hasApiKey"`
	APIKeyLast4 *string                `json:"apiKeyLast4"`
	Status      AccountStatus          `json:"status"`
	LastError   *string                `json:"lastError"`
	SignInHint  *string                `json:"signInCommand"`
	SignIn      *AccountSignInResponse `json:"signIn"`
	Models      []ModelResponse        `json:"models"`
	CreatedAt   time.Time              `json:"createdAt"`
}

type AiAccountListResponse struct {
	Accounts []AiAccountResponse `json:"accounts"`
}

func NewModelResponses(provider Provider, custom []AccountModel) []ModelResponse {
	definition, _ := FindProviderDefinition(provider)
	responses := make([]ModelResponse, 0, len(definition.Models)+len(custom))
	for _, model := range definition.Models {
		responses = append(responses, ModelResponse{ID: model.ID, DisplayName: model.DisplayName})
	}
	for _, model := range custom {
		token := model.Token
		responses = append(responses, ModelResponse{Token: &token, ID: model.ModelID, DisplayName: model.DisplayName, Custom: true})
	}
	return responses
}

// NewAiAccountResponse never carries the key or a sign-in code; it only says whether a key is set and its last characters.
// A subscription account that is not ready keeps the runner command as the fallback to the web sign-in.
func NewAiAccountResponse(detailed AccountWithModels) AiAccountResponse {
	account := detailed.Account
	response := AiAccountResponse{
		Token: account.Token, Name: account.Name, Provider: account.Provider, Mode: account.Mode, HasAPIKey: account.HasAPIKey(),
		APIKeyLast4: account.APIKeyLast4, Status: account.Status, LastError: account.LastError,
		Models: NewModelResponses(account.Provider, detailed.CustomModels), CreatedAt: account.CreatedAt,
	}
	if detailed.Runner != nil {
		response.Runner = &AccountRunnerResponse{Token: detailed.Runner.Token, Name: detailed.Runner.Name}
	}
	if detailed.SignIn != nil {
		signIn := NewAccountSignInResponse(*detailed.SignIn)
		response.SignIn = &signIn
	}
	if account.Mode == AccountModeSubscription && account.Status != AccountStatusReady {
		hint := "genwork-runner account login " + account.Token
		response.SignInHint = &hint
	}
	return response
}

type AiAccountHandler struct {
	service *AiAccountService
}

func NewAiAccountHandler(service *AiAccountService) *AiAccountHandler {
	return &AiAccountHandler{service: service}
}

func (handler *AiAccountHandler) Register(router *httpapi.Router) {
	manage := httpapi.RequirePermission(access.PermissionAccountManage)
	router.HandleFunc("GET /api/genagents/v1/providers", manage, handler.listProviders)
	router.HandleFunc("GET /api/genagents/v1/accounts", manage, handler.listAccounts)
	router.HandleFunc("POST /api/genagents/v1/accounts", manage, handler.createAccount)
	router.HandleFunc("PATCH /api/genagents/v1/accounts/{token}", manage, handler.updateAccount)
	router.HandleFunc("DELETE /api/genagents/v1/accounts/{token}", manage, handler.removeAccount)
	router.HandleFunc("POST /api/genagents/v1/accounts/{token}/models", manage, handler.addModel)
	router.HandleFunc("DELETE /api/genagents/v1/accounts/{token}/models/{modelToken}", manage, handler.removeModel)
}

func (handler *AiAccountHandler) listProviders(w http.ResponseWriter, r *http.Request) {
	response := ProviderListResponse{}
	for _, definition := range ProviderDefinitions() {
		response.Providers = append(response.Providers, ProviderResponse{
			Provider: definition.Provider, CLI: definition.CLI, Modes: definition.Modes, Models: NewModelResponses(definition.Provider, nil),
		})
	}
	httpapi.WriteJSON(w, http.StatusOK, response)
}

func (handler *AiAccountHandler) listAccounts(w http.ResponseWriter, r *http.Request) {
	listed, err := handler.service.ListAccounts(r.Context())
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	response := AiAccountListResponse{Accounts: make([]AiAccountResponse, 0, len(listed))}
	for _, detailed := range listed {
		response.Accounts = append(response.Accounts, NewAiAccountResponse(detailed))
	}
	httpapi.WriteJSON(w, http.StatusOK, response)
}

func (handler *AiAccountHandler) createAccount(w http.ResponseWriter, r *http.Request) {
	var request CreateAiAccountRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	input, err := request.Validate()
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	created, err := handler.service.CreateAccount(r.Context(), input)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, NewAiAccountResponse(created))
}

func (handler *AiAccountHandler) updateAccount(w http.ResponseWriter, r *http.Request) {
	var request UpdateAiAccountRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	changes, err := request.Validate()
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	updated, err := handler.service.UpdateAccount(r.Context(), r.PathValue("token"), changes)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, NewAiAccountResponse(updated))
}

func (handler *AiAccountHandler) removeAccount(w http.ResponseWriter, r *http.Request) {
	if err := handler.service.RemoveAccount(r.Context(), r.PathValue("token")); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler *AiAccountHandler) addModel(w http.ResponseWriter, r *http.Request) {
	var request AddAccountModelRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	input, err := request.Validate()
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	updated, err := handler.service.AddModel(r.Context(), r.PathValue("token"), input)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, NewAiAccountResponse(updated))
}

func (handler *AiAccountHandler) removeModel(w http.ResponseWriter, r *http.Request) {
	if err := handler.service.RemoveModel(r.Context(), r.PathValue("token"), r.PathValue("modelToken")); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
