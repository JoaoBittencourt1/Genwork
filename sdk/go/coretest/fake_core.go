package coretest

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/coreclient"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/manifest"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/registration"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/tokens"
	"github.com/golang-jwt/jwt/v5"
)

const (
	Issuer       = "http://genwork.test"
	ModuleSecret = "test-module-secret-0123456789abcdef"
)

type signingKey struct {
	private ed25519.PrivateKey
	keyID   string
}

// FakeCore serves the core endpoints modules depend on: JWKS, user directory, and module registration.
type FakeCore struct {
	server             *httptest.Server
	mutex              sync.Mutex
	keys               []signingKey
	users              map[string]coreclient.DirectoryUser
	registrations      []manifest.RegistrationRequest
	rejection          *rejection
	directoryFailing   bool
	jwksRequests       int
	directoryRequests  int
	failedRegistration int
}

type rejection struct {
	status int
	code   string
}

func NewFakeCore(t testing.TB) *FakeCore {
	t.Helper()
	core := &FakeCore{users: map[string]coreclient.DirectoryUser{}}
	core.keys = []signingKey{newSigningKey(t)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/core/v1/auth/jwks", core.serveJWKS)
	mux.HandleFunc("GET /api/core/v1/users/{token}", core.serveDirectoryUser)
	mux.HandleFunc("PUT /internal/v1/modules/{id}", core.serveRegistration)
	core.server = httptest.NewServer(mux)
	t.Cleanup(core.server.Close)
	return core
}

func (core *FakeCore) URL() string {
	return core.server.URL
}

func (core *FakeCore) JWKSURL() string {
	return core.server.URL + "/api/core/v1/auth/jwks"
}

func (core *FakeCore) MintAccessToken(t testing.TB, userToken string, permissions ...string) string {
	t.Helper()
	return core.MintAccessTokenExpiringAt(t, userToken, time.Now().Add(15*time.Minute), permissions...)
}

func (core *FakeCore) MintAccessTokenExpiringAt(t testing.TB, userToken string, expiresAt time.Time, permissions ...string) string {
	t.Helper()
	core.mutex.Lock()
	key := core.keys[len(core.keys)-1]
	core.mutex.Unlock()
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, tokens.AccessClaims{
		SessionToken: "fake-session",
		Permissions:  append([]string{}, permissions...),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    Issuer,
			Audience:  jwt.ClaimStrings{tokens.DefaultAudience},
			Subject:   userToken,
			IssuedAt:  jwt.NewNumericDate(expiresAt.Add(-15 * time.Minute)),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	})
	token.Header["kid"] = key.keyID
	signed, err := token.SignedString(key.private)
	if err != nil {
		t.Fatalf("sign fake access token: %v", err)
	}
	return signed
}

// RotateKey adds a new signing key; tokens minted afterwards carry a key ID verifiers have not cached yet.
func (core *FakeCore) RotateKey(t testing.TB) {
	t.Helper()
	core.mutex.Lock()
	defer core.mutex.Unlock()
	core.keys = append(core.keys, newSigningKey(t))
}

func (core *FakeCore) AddUser(user coreclient.DirectoryUser) {
	core.mutex.Lock()
	defer core.mutex.Unlock()
	core.users[user.Token] = user
}

func (core *FakeCore) FailUserDirectory() {
	core.mutex.Lock()
	defer core.mutex.Unlock()
	core.directoryFailing = true
}

func (core *FakeCore) RejectRegistrations(status int, code string) {
	core.mutex.Lock()
	defer core.mutex.Unlock()
	core.rejection = &rejection{status: status, code: code}
}

// FailNextRegistrations answers 503 to the next count registrations, as a core that is still starting would.
func (core *FakeCore) FailNextRegistrations(count int) {
	core.mutex.Lock()
	defer core.mutex.Unlock()
	core.failedRegistration = count
}

func (core *FakeCore) Registrations() []manifest.RegistrationRequest {
	core.mutex.Lock()
	defer core.mutex.Unlock()
	return append([]manifest.RegistrationRequest{}, core.registrations...)
}

func (core *FakeCore) JWKSRequests() int {
	core.mutex.Lock()
	defer core.mutex.Unlock()
	return core.jwksRequests
}

func (core *FakeCore) DirectoryRequests() int {
	core.mutex.Lock()
	defer core.mutex.Unlock()
	return core.directoryRequests
}

func (core *FakeCore) serveJWKS(w http.ResponseWriter, _ *http.Request) {
	core.mutex.Lock()
	defer core.mutex.Unlock()
	core.jwksRequests++
	keySet := tokens.JSONWebKeySet{}
	for _, key := range core.keys {
		keySet.Keys = append(keySet.Keys, tokens.JSONWebKey{
			KeyType: "OKP", Curve: "Ed25519", Algorithm: "EdDSA", Use: "sig", KeyID: key.keyID,
			X: base64.RawURLEncoding.EncodeToString(key.private.Public().(ed25519.PublicKey)),
		})
	}
	writeJSON(w, http.StatusOK, keySet)
}

func (core *FakeCore) serveDirectoryUser(w http.ResponseWriter, r *http.Request) {
	core.mutex.Lock()
	defer core.mutex.Unlock()
	core.directoryRequests++
	if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
		writeError(w, http.StatusUnauthorized, "invalid_token")
		return
	}
	if core.directoryFailing {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	user, found := core.users[r.PathValue("token")]
	if !found {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(Fixture("user_not_found.json"))
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (core *FakeCore) serveRegistration(w http.ResponseWriter, r *http.Request) {
	core.mutex.Lock()
	defer core.mutex.Unlock()
	if r.Header.Get("Authorization") != "Bearer "+ModuleSecret {
		writeError(w, http.StatusUnauthorized, "invalid_module_secret")
		return
	}
	if core.failedRegistration > 0 {
		core.failedRegistration--
		writeError(w, http.StatusServiceUnavailable, "service_unavailable")
		return
	}
	if core.rejection != nil {
		writeError(w, core.rejection.status, core.rejection.code)
		return
	}
	var request manifest.RegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.ID != r.PathValue("id") {
		writeError(w, http.StatusBadRequest, "validation_failed")
		return
	}
	core.registrations = append(core.registrations, request)
	writeJSON(w, http.StatusOK, registration.RegisteredModule{
		Token: "TW9kdWxlLXRva2VuLWV4YW0", ID: request.ID, Version: request.Version, Enabled: true,
	})
}

func newSigningKey(t testing.TB) signingKey {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate fake core key: %v", err)
	}
	fingerprint := sha256.Sum256(private.Public().(ed25519.PublicKey))
	return signingKey{private: private, keyID: base64.RawURLEncoding.EncodeToString(fingerprint[:12])}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code}})
}
