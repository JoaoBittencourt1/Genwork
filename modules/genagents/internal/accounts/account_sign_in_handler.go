package accounts

import (
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/access"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runners"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

// AccountSignInResponse shows a sign-in's progress; it never carries the pasted code.
type AccountSignInResponse struct {
	Token       string      `json:"token"`
	State       SignInState `json:"state"`
	URL         *string     `json:"url"`
	URLHost     *string     `json:"urlHost"`
	UserCode    *string     `json:"userCode"`
	ExpectsCode bool        `json:"expectsCode"`
	Failure     *string     `json:"failure"`
	ExpiresAt   time.Time   `json:"expiresAt"`
	CreatedAt   time.Time   `json:"createdAt"`
}

func NewAccountSignInResponse(signIn AccountSignIn) AccountSignInResponse {
	return AccountSignInResponse{
		Token: signIn.Token, State: signIn.State, URL: signIn.SignInURL, URLHost: signIn.SignInURLHost(), UserCode: signIn.UserCode,
		ExpectsCode: signIn.ExpectsCode, Failure: signIn.Failure, ExpiresAt: signIn.ExpiresAt, CreatedAt: signIn.CreatedAt,
	}
}

// RunnerSignInResponse is what the runner holding the account receives; Code is present only once a person pasted it.
type RunnerSignInResponse struct {
	Token        string      `json:"token"`
	AccountToken string      `json:"accountToken"`
	Provider     Provider    `json:"provider"`
	State        SignInState `json:"state"`
	Code         string      `json:"code,omitempty"`
	ExpiresAt    time.Time   `json:"expiresAt"`
}

type RunnerSignInListResponse struct {
	SignIns []RunnerSignInResponse `json:"signIns"`
}

type AccountSignInHandler struct {
	service       *AccountSignInService
	authenticator runners.RunnerAuthenticator
}

func NewAccountSignInHandler(service *AccountSignInService, authenticator runners.RunnerAuthenticator) *AccountSignInHandler {
	return &AccountSignInHandler{service: service, authenticator: authenticator}
}

func (handler *AccountSignInHandler) Register(router *httpapi.Router) {
	manage := httpapi.RequirePermission(access.PermissionAccountManage)
	router.HandleFunc("POST /api/genagents/v1/accounts/{token}/sign-ins", manage, handler.startSignIn)
	router.HandleFunc("POST /api/genagents/v1/accounts/{token}/sign-ins/{signInToken}/code", manage, handler.submitCode)
	router.HandleFunc("DELETE /api/genagents/v1/accounts/{token}/sign-ins/{signInToken}", manage, handler.cancelSignIn)
	router.HandleFunc("GET /api/genagents/v1/runner/sign-ins", httpapi.Public, runners.RequireRunner(handler.authenticator, handler.listForRunner))
	router.HandleFunc("POST /api/genagents/v1/runner/sign-ins/{signInToken}/progress", httpapi.Public, runners.RequireRunner(handler.authenticator, handler.reportProgress))
}

func (handler *AccountSignInHandler) startSignIn(w http.ResponseWriter, r *http.Request) {
	signIn, err := handler.service.StartSignIn(r.Context(), r.PathValue("token"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, NewAccountSignInResponse(signIn))
}

func (handler *AccountSignInHandler) submitCode(w http.ResponseWriter, r *http.Request) {
	var request SubmitSignInCodeRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	code, err := request.Validate()
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	signIn, err := handler.service.SubmitCode(r.Context(), r.PathValue("token"), r.PathValue("signInToken"), code)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, NewAccountSignInResponse(signIn))
}

func (handler *AccountSignInHandler) cancelSignIn(w http.ResponseWriter, r *http.Request) {
	if err := handler.service.CancelSignIn(r.Context(), r.PathValue("token"), r.PathValue("signInToken")); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler *AccountSignInHandler) listForRunner(w http.ResponseWriter, r *http.Request, runner runners.Runner) {
	listed, err := handler.service.ListForRunner(r.Context(), runner)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	response := RunnerSignInListResponse{SignIns: make([]RunnerSignInResponse, 0, len(listed))}
	for _, signIn := range listed {
		response.SignIns = append(response.SignIns, RunnerSignInResponse{
			Token: signIn.Token, AccountToken: signIn.AccountToken, Provider: signIn.Provider, State: signIn.State, Code: signIn.Code, ExpiresAt: signIn.ExpiresAt,
		})
	}
	w.Header().Set("Cache-Control", "no-store")
	httpapi.WriteJSON(w, http.StatusOK, response)
}

func (handler *AccountSignInHandler) reportProgress(w http.ResponseWriter, r *http.Request, runner runners.Runner) {
	var request ReportSignInProgressRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	progress, err := request.Validate()
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	if err := handler.service.ReportProgress(r.Context(), runner, r.PathValue("signInToken"), progress); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
