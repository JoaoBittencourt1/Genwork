package mcpservers

import (
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/access"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

// EntryResponse never carries a secret value: it says whether one is set.
type EntryResponse struct {
	Name     string  `json:"name"`
	Secret   bool    `json:"secret"`
	HasValue bool    `json:"hasValue"`
	Value    *string `json:"value"`
}

type McpServerResponse struct {
	Token     string          `json:"token"`
	Name      string          `json:"name"`
	Transport Transport       `json:"transport"`
	URL       *string         `json:"url"`
	Headers   []EntryResponse `json:"headers"`
	Command   *string         `json:"command"`
	Args      []string        `json:"args"`
	Env       []EntryResponse `json:"env"`
	// OAuth is nil while the server was never connected through OAuth.
	OAuth     *OAuthConnectionResponse `json:"oauth"`
	UpdatedAt time.Time                `json:"updatedAt"`
}

type McpServerListResponse struct {
	Servers []McpServerResponse `json:"servers"`
}

func newEntryResponses(entries []Entry) []EntryResponse {
	responses := make([]EntryResponse, 0, len(entries))
	for _, entry := range entries {
		response := EntryResponse{Name: entry.Name, Secret: entry.Secret, HasValue: entry.Secret || entry.Value != ""}
		if !entry.Secret {
			value := entry.Value
			response.Value = &value
		}
		responses = append(responses, response)
	}
	return responses
}

func NewMcpServerResponse(server McpServer) McpServerResponse {
	response := McpServerResponse{
		Token: server.Token, Name: server.Name, Transport: server.Transport, Headers: newEntryResponses(server.Config.Headers),
		Args: server.Config.Args, Env: newEntryResponses(server.Config.Env), UpdatedAt: server.UpdatedAt,
	}
	if response.Args == nil {
		response.Args = []string{}
	}
	if server.OAuth != nil {
		connection := NewOAuthConnectionResponse(*server.OAuth)
		response.OAuth = &connection
	}
	if server.Transport == TransportHTTP {
		response.URL = &server.Config.URL
	} else {
		response.Command = &server.Config.Command
	}
	return response
}

type McpServerHandler struct {
	service *McpServerService
}

func NewMcpServerHandler(service *McpServerService) *McpServerHandler {
	return &McpServerHandler{service: service}
}

func (handler *McpServerHandler) Register(router *httpapi.Router) {
	manage := httpapi.RequirePermission(access.PermissionMCPServerManage)
	router.HandleFunc("GET /api/genagents/v1/mcp-servers", manage, handler.listServers)
	router.HandleFunc("POST /api/genagents/v1/mcp-servers", manage, handler.createServer)
	router.HandleFunc("PATCH /api/genagents/v1/mcp-servers/{token}", manage, handler.updateServer)
	router.HandleFunc("DELETE /api/genagents/v1/mcp-servers/{token}", manage, handler.deleteServer)
}

func (handler *McpServerHandler) listServers(w http.ResponseWriter, r *http.Request) {
	listed, err := handler.service.ListServers(r.Context())
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	response := McpServerListResponse{Servers: make([]McpServerResponse, 0, len(listed))}
	for _, server := range listed {
		response.Servers = append(response.Servers, NewMcpServerResponse(server))
	}
	httpapi.WriteJSON(w, http.StatusOK, response)
}

func (handler *McpServerHandler) createServer(w http.ResponseWriter, r *http.Request) {
	var request CreateMcpServerRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	server, err := request.Validate()
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	created, err := handler.service.CreateServer(r.Context(), server)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, NewMcpServerResponse(created))
}

func (handler *McpServerHandler) updateServer(w http.ResponseWriter, r *http.Request) {
	var request UpdateMcpServerRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	updated, err := handler.service.UpdateServer(r.Context(), r.PathValue("token"), request)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, NewMcpServerResponse(updated))
}

func (handler *McpServerHandler) deleteServer(w http.ResponseWriter, r *http.Request) {
	if err := handler.service.DeleteServer(r.Context(), r.PathValue("token")); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
