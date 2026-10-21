package auth

import (
	"errors"
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/core/internal/sessions"
)

const (
	RefreshCookieName = "genwork_refresh"
	refreshCookiePath = "/api/core/v1/auth"
	ClientHeader      = "X-Genwork-Client"
)

type Middleware func(http.Handler) http.Handler

type LoginRequest struct {
	Email      string  `json:"email"`
	Password   string  `json:"password"`
	Client     string  `json:"client"`
	DeviceName *string `json:"deviceName"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type SignInResponse struct {
	AccessToken          string    `json:"accessToken"`
	AccessTokenExpiresAt time.Time `json:"accessTokenExpiresAt"`
	RefreshToken         string    `json:"refreshToken,omitempty"`
	Session              string    `json:"session"`
}

type SessionResponse struct {
	Token      string    `json:"token"`
	ClientType string    `json:"clientType"`
	DeviceName *string   `json:"deviceName"`
	CreatedAt  time.Time `json:"createdAt"`
	LastUsedAt time.Time `json:"lastUsedAt"`
	Current    bool      `json:"current"`
}

type SessionListResponse struct {
	Sessions []SessionResponse `json:"sessions"`
}

type Handler struct {
	service        *Service
	sessions       *sessions.Service
	signingKey     SigningKey
	loginLimiter   Middleware
	refreshLimiter Middleware
}

func NewHandler(service *Service, sessionService *sessions.Service, signingKey SigningKey, loginLimiter, refreshLimiter Middleware) *Handler {
	return &Handler{
		service: service, sessions: sessionService, signingKey: signingKey,
		loginLimiter: loginLimiter, refreshLimiter: refreshLimiter,
	}
}

func (handler *Handler) Register(router *platform.Router) {
	router.HandleFunc("GET /api/core/v1/auth/jwks", platform.Public, handler.jwks)
	router.Handle("POST /api/core/v1/auth/login", platform.Public, handler.loginLimiter(http.HandlerFunc(handler.login)))
	router.Handle("POST /api/core/v1/auth/refresh", platform.Public, handler.refreshLimiter(http.HandlerFunc(handler.refresh)))
	router.HandleFunc("POST /api/core/v1/auth/logout", platform.Authenticated, handler.logout)
	router.HandleFunc("GET /api/core/v1/auth/sessions", platform.Authenticated, handler.listSessions)
	router.HandleFunc("DELETE /api/core/v1/auth/sessions/{token}", platform.Authenticated, handler.revokeSession)
}

func (handler *Handler) jwks(w http.ResponseWriter, r *http.Request) {
	platform.WriteJSON(w, http.StatusOK, handler.signingKey.JWKSet())
}

func (handler *Handler) login(w http.ResponseWriter, r *http.Request) {
	var request LoginRequest
	if err := platform.DecodeJSON(w, r, &request); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	signIn, err := handler.service.Login(r.Context(), LoginInput{
		Email:       request.Email,
		Password:    request.Password,
		ClientInput: ClientInput{Client: sessions.ClientType(request.Client), DeviceName: request.DeviceName},
	})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	WriteSignIn(w, signIn)
}

func (handler *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	refreshToken, err := readRefreshToken(w, r)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	signIn, err := handler.service.Refresh(r.Context(), refreshToken)
	if err != nil {
		if isWebClient(r) {
			clearRefreshCookie(w)
		}
		platform.WriteError(w, r, err)
		return
	}
	WriteSignIn(w, signIn)
}

func (handler *Handler) logout(w http.ResponseWriter, r *http.Request) {
	principal, _ := platform.PrincipalFrom(r.Context())
	if err := handler.sessions.Revoke(r.Context(), principal.SessionID); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	if isWebClient(r) {
		clearRefreshCookie(w)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler *Handler) listSessions(w http.ResponseWriter, r *http.Request) {
	principal, _ := platform.PrincipalFrom(r.Context())
	activeSessions, err := handler.sessions.ListActiveForUser(r.Context(), principal.UserID)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	response := SessionListResponse{Sessions: make([]SessionResponse, 0, len(activeSessions))}
	for _, session := range activeSessions {
		response.Sessions = append(response.Sessions, SessionResponse{
			Token:      session.Token,
			ClientType: string(session.ClientType),
			DeviceName: session.DeviceName,
			CreatedAt:  session.CreatedAt,
			LastUsedAt: session.LastUsedAt,
			Current:    session.ID == principal.SessionID,
		})
	}
	platform.WriteJSON(w, http.StatusOK, response)
}

func (handler *Handler) revokeSession(w http.ResponseWriter, r *http.Request) {
	principal, _ := platform.PrincipalFrom(r.Context())
	if err := handler.sessions.RevokeForUser(r.Context(), principal.UserID, r.PathValue("token")); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// WriteSignIn delivers the refresh token as an HttpOnly cookie to the web app and in the body to native clients.
func WriteSignIn(w http.ResponseWriter, signIn SignIn) {
	response := SignInResponse{
		AccessToken:          signIn.AccessToken,
		AccessTokenExpiresAt: signIn.AccessTokenExpiresAt,
		Session:              signIn.Session.Token,
	}
	if signIn.Session.ClientType == sessions.ClientWeb {
		http.SetCookie(w, &http.Cookie{
			Name:     RefreshCookieName,
			Value:    signIn.RefreshToken,
			Path:     refreshCookiePath,
			Expires:  signIn.Session.AbsoluteExpiresAt,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
		})
	} else {
		response.RefreshToken = signIn.RefreshToken
	}
	platform.WriteJSON(w, http.StatusOK, response)
}

// readRefreshToken accepts the cookie only alongside the X-Genwork-Client header, which a cross-site form cannot send.
func readRefreshToken(w http.ResponseWriter, r *http.Request) (string, error) {
	if isWebClient(r) {
		cookie, err := r.Cookie(RefreshCookieName)
		if errors.Is(err, http.ErrNoCookie) || cookie.Value == "" {
			return "", platform.ErrInvalidToken
		}
		return cookie.Value, nil
	}
	var request RefreshRequest
	if err := platform.DecodeJSON(w, r, &request); err != nil {
		return "", err
	}
	if request.RefreshToken == "" {
		return "", platform.ErrInvalidToken
	}
	return request.RefreshToken, nil
}

func isWebClient(r *http.Request) bool {
	return r.Header.Get(ClientHeader) == string(sessions.ClientWeb)
}

func clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    "",
		Path:     refreshCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}
