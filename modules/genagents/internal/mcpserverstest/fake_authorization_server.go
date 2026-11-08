// Package mcpserverstest stubs an MCP server protected by OAuth and its authorization server, so the OAuth flow is tested
// without any real provider (rule 25).
package mcpserverstest

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
)

const (
	RegisteredClientID = "dynamic-client-1"
	ManualClientID     = "manual-client-1"
	ManualClientSecret = "manual-client-secret-never-returned"
)

// FakeAuthorizationServer is an MCP server at /mcp whose authorization server lives on the same origin.
type FakeAuthorizationServer struct {
	server *httptest.Server
	mutex  sync.Mutex
	// WithoutRegistration hides the registration endpoint, as a server that needs a client registered by hand does.
	WithoutRegistration bool
	// FailRefresh makes refresh grants fail with invalid_grant.
	FailRefresh   bool
	challenges    map[string]string
	issued        int
	Registrations int
	// TokenRequests records every token request's form, in order.
	TokenRequests []map[string]string
	// BasicAuthClients records the client ids sent with HTTP Basic authentication.
	BasicAuthClients []string
}

func NewFakeAuthorizationServer(t testing.TB) *FakeAuthorizationServer {
	t.Helper()
	fake := &FakeAuthorizationServer{challenges: map[string]string{}}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /mcp", fake.serveUnauthorized)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource/mcp", fake.serveResourceMetadata)
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", fake.serveServerMetadata)
	mux.HandleFunc("POST /register", fake.serveRegistration)
	mux.HandleFunc("POST /token", fake.serveToken)
	fake.server = httptest.NewServer(mux)
	t.Cleanup(fake.server.Close)
	return fake
}

func (fake *FakeAuthorizationServer) URL() string { return fake.server.URL }

func (fake *FakeAuthorizationServer) MCPURL() string { return fake.server.URL + "/mcp" }

// Approve plays the person consenting on the authorization page: it returns the code the browser brings back.
func (fake *FakeAuthorizationServer) Approve(codeChallenge string) string {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.issued++
	code := "authorization-code-" + strconv.Itoa(fake.issued)
	fake.challenges[code] = codeChallenge
	return code
}

func (fake *FakeAuthorizationServer) Tokens() []map[string]string {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	return append([]map[string]string(nil), fake.TokenRequests...)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (fake *FakeAuthorizationServer) serveUnauthorized(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+fake.server.URL+`/.well-known/oauth-protected-resource/mcp"`)
	w.WriteHeader(http.StatusUnauthorized)
}

func (fake *FakeAuthorizationServer) serveResourceMetadata(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"resource": fake.MCPURL(), "authorization_servers": []string{fake.server.URL}, "scopes_supported": []string{"mcp:tools"}})
}

func (fake *FakeAuthorizationServer) serveServerMetadata(w http.ResponseWriter, _ *http.Request) {
	metadata := map[string]any{
		"issuer": fake.server.URL, "authorization_endpoint": fake.server.URL + "/authorize", "token_endpoint": fake.server.URL + "/token",
		"code_challenge_methods_supported": []string{"S256"},
	}
	if !fake.WithoutRegistration {
		metadata["registration_endpoint"] = fake.server.URL + "/register"
	}
	writeJSON(w, http.StatusOK, metadata)
}

func (fake *FakeAuthorizationServer) serveRegistration(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	fake.mutex.Lock()
	fake.Registrations++
	fake.mutex.Unlock()
	writeJSON(w, http.StatusCreated, map[string]any{"client_id": RegisteredClientID, "redirect_uris": body["redirect_uris"], "token_endpoint_auth_method": "none"})
}

// serveToken checks PKCE for an authorization code and rotates tokens on refresh.
func (fake *FakeAuthorizationServer) serveToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_request"})
		return
	}
	form := map[string]string{}
	for key := range r.PostForm {
		form[key] = r.PostForm.Get(key)
	}
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.TokenRequests = append(fake.TokenRequests, form)
	if client, secret, ok := r.BasicAuth(); ok {
		if client != ManualClientID || secret != ManualClientSecret {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid_client"})
			return
		}
		fake.BasicAuthClients = append(fake.BasicAuthClients, client)
	}
	switch form["grant_type"] {
	case "authorization_code":
		challenge, found := fake.challenges[form["code"]]
		delete(fake.challenges, form["code"])
		digest := sha256.Sum256([]byte(form["code_verifier"]))
		if !found || base64.RawURLEncoding.EncodeToString(digest[:]) != challenge {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"access_token": "mcp-access-1", "refresh_token": "mcp-refresh-1", "expires_in": 3600, "token_type": "Bearer"})
	case "refresh_token":
		if fake.FailRefresh || form["refresh_token"] != "mcp-refresh-1" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"access_token": "mcp-access-2", "expires_in": 3600, "token_type": "Bearer"})
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "unsupported_grant_type"})
	}
}
