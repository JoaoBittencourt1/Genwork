package githubsynctest

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// IssuedInstallationToken is the fixture token the fake hands out for every installation token request.
const IssuedInstallationToken = "ghs_fixtureInstallationToken0123456789"

// InstallationTokenRequest records one installation token request.
type InstallationTokenRequest struct {
	InstallationID int64
	Repositories   []string
	Permissions    map[string]string
}

// UseApp makes the fake verify App JWTs with the App's public key and answer installation lookups from installations,
// which maps a lowercase "owner/name" to an installation id; GitHub matches repository names case-insensitively.
func (github *FakeGithub) UseApp(publicKey *rsa.PublicKey, appID int64, installations map[string]int64) {
	github.mutex.Lock()
	defer github.mutex.Unlock()
	github.appKey, github.appID, github.installations = publicKey, appID, installations
}

// RefuseTokenPermissions makes token requests fail as they do for an App whose owner has not accepted new permissions.
func (github *FakeGithub) RefuseTokenPermissions() {
	github.mutex.Lock()
	defer github.mutex.Unlock()
	github.refusePermissions = true
}

func (github *FakeGithub) TokenRequests() []InstallationTokenRequest {
	github.mutex.Lock()
	defer github.mutex.Unlock()
	return append([]InstallationTokenRequest(nil), github.tokenRequests...)
}

func (github *FakeGithub) registerInstallationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /repos/{owner}/{name}/installation", github.serveInstallation)
	mux.HandleFunc("POST /app/installations/{id}/access_tokens", github.serveAccessToken)
}

func writeGithubJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (github *FakeGithub) serveInstallation(w http.ResponseWriter, r *http.Request) {
	if !github.validAppJWT(r) {
		writeGithubJSON(w, http.StatusUnauthorized, map[string]string{"message": "A JSON web token could not be decoded"})
		return
	}
	github.mutex.Lock()
	installationID, found := github.installations[strings.ToLower(r.PathValue("owner")+"/"+r.PathValue("name"))]
	github.mutex.Unlock()
	if !found {
		writeGithubJSON(w, http.StatusNotFound, map[string]string{"message": "Not Found"})
		return
	}
	writeGithubJSON(w, http.StatusOK, map[string]any{"id": installationID, "app_id": github.appID})
}

func (github *FakeGithub) serveAccessToken(w http.ResponseWriter, r *http.Request) {
	if !github.validAppJWT(r) {
		writeGithubJSON(w, http.StatusUnauthorized, map[string]string{"message": "A JSON web token could not be decoded"})
		return
	}
	var body struct {
		Repositories []string          `json:"repositories"`
		Permissions  map[string]string `json:"permissions"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	installationID, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	github.mutex.Lock()
	github.tokenRequests = append(github.tokenRequests, InstallationTokenRequest{InstallationID: installationID, Repositories: body.Repositories, Permissions: body.Permissions})
	refuse := github.refusePermissions
	github.mutex.Unlock()
	if refuse {
		writeGithubJSON(w, http.StatusUnprocessableEntity, map[string]string{"message": "The permissions requested are not granted to this installation."})
		return
	}
	writeGithubJSON(w, http.StatusCreated, map[string]any{"token": IssuedInstallationToken, "expires_at": "2026-11-05T13:00:00Z"})
}

// validAppJWT checks the RS256 signature and the issuer the way GitHub does.
func (github *FakeGithub) validAppJWT(r *http.Request) bool {
	token, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	parts := strings.Split(token, ".")
	github.mutex.Lock()
	publicKey, appID := github.appKey, github.appID
	github.mutex.Unlock()
	if !found || len(parts) != 3 || publicKey == nil {
		return false
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature) != nil {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	var claims struct {
		Issuer string `json:"iss"`
	}
	return json.Unmarshal(payload, &claims) == nil && claims.Issuer == strconv.FormatInt(appID, 10)
}
