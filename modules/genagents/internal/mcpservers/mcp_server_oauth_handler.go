package mcpservers

import (
	"net/http"
	"strings"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/access"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

// OAuthConnectionResponse shows a connection's state; it never carries a token or the client secret.
type OAuthConnectionResponse struct {
	Status      OAuthStatus `json:"status"`
	ClientID    *string     `json:"clientId"`
	ConnectedAt *time.Time  `json:"connectedAt"`
	ExpiresAt   *time.Time  `json:"expiresAt"`
	LastError   *string     `json:"lastError"`
}

func NewOAuthConnectionResponse(connection OAuthConnection) OAuthConnectionResponse {
	return OAuthConnectionResponse{
		Status: connection.Status, ClientID: optionalString(connection.Setup.ClientID), ConnectedAt: connection.ConnectedAt, ExpiresAt: connection.ExpiresAt, LastError: connection.LastError,
	}
}

type StartOAuthRequest struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	Scope        string `json:"scope"`
}

func (request StartOAuthRequest) Validate() (StartOAuthInput, error) {
	input := StartOAuthInput{ClientID: strings.TrimSpace(request.ClientID), ClientSecret: request.ClientSecret, Scope: strings.TrimSpace(request.Scope)}
	fields := map[string]string{}
	if len(input.ClientID) > MaxOAuthClientIDLength {
		fields["clientId"] = "invalid"
	}
	if len(input.ClientSecret) > MaxFieldBytes || (input.ClientSecret != "" && input.ClientID == "") {
		fields["clientSecret"] = "invalid"
	}
	if len(input.Scope) > MaxOAuthScopeLength {
		fields["scope"] = "invalid"
	}
	if len(fields) > 0 {
		return StartOAuthInput{}, httpapi.NewValidationError(fields)
	}
	return input, nil
}

type OAuthAuthorizationResponse struct {
	AuthorizationURL string `json:"authorizationUrl"`
}

type McpServerOAuthHandler struct {
	service *McpServerOAuthService
}

func NewMcpServerOAuthHandler(service *McpServerOAuthService) *McpServerOAuthHandler {
	return &McpServerOAuthHandler{service: service}
}

// Register declares the callback public: the browser arrives from the authorization server without a Genwork token, and
// the single-use state is what authorizes it (rule 18).
func (handler *McpServerOAuthHandler) Register(router *httpapi.Router) {
	manage := httpapi.RequirePermission(access.PermissionMCPServerManage)
	router.HandleFunc("POST /api/genagents/v1/mcp-servers/{token}/oauth/authorizations", manage, handler.startAuthorization)
	router.HandleFunc("DELETE /api/genagents/v1/mcp-servers/{token}/oauth", manage, handler.disconnect)
	router.HandleFunc("GET "+OAuthCallbackPath, httpapi.Public, handler.completeAuthorization)
}

func (handler *McpServerOAuthHandler) startAuthorization(w http.ResponseWriter, r *http.Request) {
	var request StartOAuthRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	input, err := request.Validate()
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	authorizationURL, err := handler.service.StartAuthorization(r.Context(), r.PathValue("token"), input)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpapi.WriteJSON(w, http.StatusCreated, OAuthAuthorizationResponse{AuthorizationURL: authorizationURL})
}

func (handler *McpServerOAuthHandler) disconnect(w http.ResponseWriter, r *http.Request) {
	if err := handler.service.Disconnect(r.Context(), r.PathValue("token")); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler *McpServerOAuthHandler) completeAuthorization(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	outcome := handler.service.CompleteAuthorization(r.Context(), query.Get("state"), query.Get("code"), query.Get("error"))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	http.Redirect(w, r, handler.service.ResultPageURL(outcome), http.StatusSeeOther)
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
