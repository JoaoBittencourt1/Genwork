package githubsync

import (
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/access"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

// RepositoryTokenResponse carries the token once; it is sent with no-store and never kept.
type RepositoryTokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type RepositoryTokenHandler struct {
	service *RepositoryTokenService
}

func NewRepositoryTokenHandler(service *RepositoryTokenService) *RepositoryTokenHandler {
	return &RepositoryTokenHandler{service: service}
}

func (handler *RepositoryTokenHandler) Register(router *httpapi.Router) {
	router.HandleFunc("POST /api/gentasks/v1/github/repository-tokens", httpapi.RequirePermission(access.PermissionRepositoryWrite), handler.issueToken)
}

func (handler *RepositoryTokenHandler) issueToken(w http.ResponseWriter, r *http.Request) {
	var request IssueRepositoryTokenRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	repositories, err := request.Validate()
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	principal, _ := httpapi.PrincipalFrom(r.Context())
	caller := TokenCaller{UserToken: principal.UserToken, AccessToken: principal.AccessToken}
	token, err := handler.service.IssueToken(r.Context(), caller, repositories)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpapi.WriteJSON(w, http.StatusOK, RepositoryTokenResponse{Token: string(token.Token.Reveal()), ExpiresAt: token.ExpiresAt})
}
