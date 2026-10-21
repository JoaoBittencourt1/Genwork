package authtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform/apitest"
)

const (
	Email    = "ada@example.test"
	Password = "correct horse battery staple"
	Session  = "ses_mcp"
)

// FakeSessionCore mimics core's session rules: refresh tokens rotate, and presenting a used one revokes the session.
type FakeSessionCore struct {
	mutex               sync.Mutex
	AccessTokenLifetime time.Duration
	Now                 func() time.Time
	issued              int
	currentRefreshToken string
	revoked             bool
	Refreshes           int
	LoggedOut           bool
	LastLogin           map[string]any
}

func NewFakeSessionCore(api *apitest.FakeAPI) *FakeSessionCore {
	core := &FakeSessionCore{AccessTokenLifetime: 15 * time.Minute, Now: time.Now}
	api.Handle(http.MethodPost, "/api/core/v1/auth/login", core.login)
	api.Handle(http.MethodPost, "/api/core/v1/auth/refresh", core.refresh)
	api.Handle(http.MethodPost, "/api/core/v1/auth/logout", core.logout)
	return core
}

func (core *FakeSessionCore) Revoke() {
	core.mutex.Lock()
	defer core.mutex.Unlock()
	core.revoked = true
}

func (core *FakeSessionCore) IsRevoked() bool {
	core.mutex.Lock()
	defer core.mutex.Unlock()
	return core.revoked
}

func (core *FakeSessionCore) login(w http.ResponseWriter, r *http.Request) {
	var request map[string]any
	_ = json.NewDecoder(r.Body).Decode(&request)
	core.mutex.Lock()
	defer core.mutex.Unlock()
	core.LastLogin = request
	if request["email"] != Email || request["password"] != Password {
		writeError(w, http.StatusUnauthorized, "invalid_credentials")
		return
	}
	core.revoked = false
	core.writeSignIn(w)
}

func (core *FakeSessionCore) refresh(w http.ResponseWriter, r *http.Request) {
	var request struct {
		RefreshToken string `json:"refreshToken"`
	}
	_ = json.NewDecoder(r.Body).Decode(&request)
	core.mutex.Lock()
	defer core.mutex.Unlock()
	if request.RefreshToken != core.currentRefreshToken {
		core.revoked = true
	}
	if core.revoked {
		writeError(w, http.StatusUnauthorized, "session_revoked")
		return
	}
	core.Refreshes++
	core.writeSignIn(w)
}

func (core *FakeSessionCore) logout(w http.ResponseWriter, r *http.Request) {
	core.mutex.Lock()
	defer core.mutex.Unlock()
	if r.Header.Get("Authorization") != fmt.Sprintf("Bearer access-%d", core.issued) {
		writeError(w, http.StatusUnauthorized, "invalid_token")
		return
	}
	core.LoggedOut = true
	core.revoked = true
	w.WriteHeader(http.StatusNoContent)
}

func (core *FakeSessionCore) writeSignIn(w http.ResponseWriter) {
	core.issued++
	core.currentRefreshToken = fmt.Sprintf("refresh-%d", core.issued)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"accessToken":          fmt.Sprintf("access-%d", core.issued),
		"accessTokenExpiresAt": core.Now().Add(core.AccessTokenLifetime),
		"refreshToken":         core.currentRefreshToken,
		"session":              Session,
	})
}

func writeError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code}})
}
