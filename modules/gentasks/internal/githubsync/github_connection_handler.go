package githubsync

import (
	"net/http"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/access"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

// GithubConnectionResponse never carries the webhook secret or the private key; App fields are absent while disconnected.
type GithubConnectionResponse struct {
	State      ConnectionState `json:"state"`
	WebhookURL string          `json:"webhookUrl"`
	AppSlug    *string         `json:"appSlug,omitempty"`
	OwnerLogin *string         `json:"ownerLogin,omitempty"`
	AppURL     *string         `json:"appUrl,omitempty"`
	InstallURL *string         `json:"installUrl,omitempty"`
}

type ManifestStartResponse struct {
	FormAction string      `json:"formAction"`
	Manifest   AppManifest `json:"manifest"`
	State      string      `json:"state"`
}

func NewGithubConnectionResponse(connection GithubConnection) GithubConnectionResponse {
	response := GithubConnectionResponse{State: connection.State, WebhookURL: connection.WebhookURL}
	if connection.App != nil {
		response.AppSlug, response.OwnerLogin, response.AppURL = &connection.App.Slug, &connection.App.OwnerLogin, &connection.App.AppURL
		response.InstallURL = &connection.InstallURL
	}
	return response
}

type GithubConnectionHandler struct {
	service *GithubConnectionService
}

func NewGithubConnectionHandler(service *GithubConnectionService) *GithubConnectionHandler {
	return &GithubConnectionHandler{service: service}
}

// Register: reading the connection only needs board access, because the board settings screen shows it and it holds no secret.
func (handler *GithubConnectionHandler) Register(router *httpapi.Router) {
	router.HandleFunc("GET /api/gentasks/v1/github/connection", httpapi.RequirePermission(access.PermissionBoardRead), handler.findConnection)
	router.HandleFunc("POST /api/gentasks/v1/github/connection/manifest", httpapi.RequirePermission(access.PermissionGithubManage), handler.startManifest)
	router.HandleFunc("POST /api/gentasks/v1/github/connection", httpapi.RequirePermission(access.PermissionGithubManage), handler.completeConnection)
	router.HandleFunc("DELETE /api/gentasks/v1/github/connection", httpapi.RequirePermission(access.PermissionGithubManage), handler.disconnect)
}

func callerTokenFrom(r *http.Request) string {
	principal, _ := httpapi.PrincipalFrom(r.Context())
	return principal.UserToken
}

func (handler *GithubConnectionHandler) findConnection(w http.ResponseWriter, r *http.Request) {
	connection, err := handler.service.FindConnection(r.Context())
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, NewGithubConnectionResponse(connection))
}

func (handler *GithubConnectionHandler) startManifest(w http.ResponseWriter, r *http.Request) {
	var request StartManifestRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	input, err := request.Validate()
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	started, err := handler.service.StartManifest(r.Context(), callerTokenFrom(r), input)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, ManifestStartResponse{FormAction: started.FormAction, Manifest: started.Manifest, State: started.State})
}

func (handler *GithubConnectionHandler) completeConnection(w http.ResponseWriter, r *http.Request) {
	var request CompleteConnectionRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	input, err := request.Validate()
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	connection, err := handler.service.CompleteConnection(r.Context(), callerTokenFrom(r), input)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, NewGithubConnectionResponse(connection))
}

func (handler *GithubConnectionHandler) disconnect(w http.ResponseWriter, r *http.Request) {
	if err := handler.service.Disconnect(r.Context()); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
