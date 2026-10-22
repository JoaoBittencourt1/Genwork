package oauth

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/auth"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
)

const maxProtocolBodyBytes = 64 << 10

type OAuthHandler struct {
	service             *OAuthService
	publicURL           string
	registrationLimiter auth.Middleware
	tokenLimiter        auth.Middleware
	now                 func() time.Time
}

func NewOAuthHandler(service *OAuthService, publicURL string, registrationLimiter, tokenLimiter auth.Middleware, now func() time.Time) *OAuthHandler {
	return &OAuthHandler{service: service, publicURL: publicURL, registrationLimiter: registrationLimiter, tokenLimiter: tokenLimiter, now: now}
}

// Register declares each route's rule (rule 18): discovery, registration, and token are public because assistants call
// them before any sign-in; the consent endpoints need the signed-in user.
func (handler *OAuthHandler) Register(router *platform.Router) {
	router.HandleFunc("GET /.well-known/oauth-authorization-server", platform.Public, handler.serveMetadata)
	router.Handle("POST /api/core/v1/oauth/register", platform.Public, handler.registrationLimiter(http.HandlerFunc(handler.registerClient)))
	router.HandleFunc("GET /api/core/v1/oauth/authorization-requests", platform.Authenticated, handler.previewAuthorization)
	router.HandleFunc("POST /api/core/v1/oauth/authorizations", platform.Authenticated, handler.decideAuthorization)
	router.Handle("POST /api/core/v1/oauth/token", platform.Public, handler.tokenLimiter(http.HandlerFunc(handler.issueToken)))
}

func (handler *OAuthHandler) serveMetadata(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=300")
	platform.WriteJSON(w, http.StatusOK, newAuthorizationServerMetadataResponse(handler.publicURL))
}

func (handler *OAuthHandler) registerClient(w http.ResponseWriter, r *http.Request) {
	var request RegisterClientRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, maxProtocolBodyBytes)).Decode(&request); err != nil {
		writeProtocolError(w, newProtocolError("invalid_client_metadata", "the body is not JSON client metadata"))
		return
	}
	if err := request.Validate(); err != nil {
		writeProtocolError(w, err)
		return
	}
	client, err := handler.service.RegisterClient(r.Context(), request)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WriteJSON(w, http.StatusCreated, newClientRegistrationResponse(client))
}

func (handler *OAuthHandler) previewAuthorization(w http.ResponseWriter, r *http.Request) {
	preview, err := handler.service.PreviewAuthorization(r.Context(), AuthorizationRequestFromQuery(r.URL.Query()))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WriteJSON(w, http.StatusOK, AuthorizationPreviewResponse{ClientName: preview.ClientName, Scope: preview.Scope, RedirectURL: preview.RedirectURL})
}

func (handler *OAuthHandler) decideAuthorization(w http.ResponseWriter, r *http.Request) {
	var request AuthorizationDecisionRequest
	if err := platform.DecodeJSON(w, r, &request); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	if fields := request.Validate(); fields != nil {
		platform.WriteError(w, r, platform.NewValidationError(fields))
		return
	}
	principal, _ := platform.PrincipalFrom(r.Context())
	redirectURL, err := handler.service.DecideAuthorization(r.Context(), principal.UserID, request.AuthorizationRequest(), request.Decision)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WriteJSON(w, http.StatusOK, AuthorizationRedirectResponse{RedirectURL: redirectURL})
}

// issueToken answers with no-store on success and failure alike: the body may carry tokens (RFC 6749 §5.1).
func (handler *OAuthHandler) issueToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	request, err := readTokenRequest(w, r)
	if err != nil {
		writeProtocolError(w, err)
		return
	}
	var signIn auth.SignIn
	if request.GrantType == grantAuthorizationCode {
		signIn, err = handler.service.ExchangeCode(r.Context(), request)
	} else {
		signIn, err = handler.service.Refresh(r.Context(), request)
	}
	var protocolError *ProtocolError
	if errors.As(err, &protocolError) {
		writeProtocolError(w, protocolError)
		return
	}
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WriteJSON(w, http.StatusOK, newTokenResponse(signIn, handler.now()))
}

func readTokenRequest(w http.ResponseWriter, r *http.Request) (TokenRequest, error) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/x-www-form-urlencoded" {
		return TokenRequest{}, newProtocolError("invalid_request", "send the request as application/x-www-form-urlencoded")
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxProtocolBodyBytes)
	if err := r.ParseForm(); err != nil {
		return TokenRequest{}, newProtocolError("invalid_request", "the form cannot be read")
	}
	return TokenRequestFromForm(r.PostForm)
}

func writeProtocolError(w http.ResponseWriter, err error) {
	var protocolError *ProtocolError
	if !errors.As(err, &protocolError) {
		protocolError = newProtocolError("invalid_request", "the request is invalid")
	}
	platform.WriteJSON(w, protocolError.Status, protocolErrorResponse{Error: protocolError.Code, ErrorDescription: protocolError.Description})
}
