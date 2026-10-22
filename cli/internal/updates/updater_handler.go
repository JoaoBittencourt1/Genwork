package updates

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/JoaoBittencourt1/Genwork/cli/internal/releases"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
	"github.com/Masterminds/semver/v3"
)

var errInvalidUpdaterSecret = httpapi.NewAPIError(http.StatusUnauthorized, "invalid_updater_secret")

// StartUpdateRequest is the body of POST /v1/updates.
type StartUpdateRequest struct {
	Version string `json:"version"`
}

func (request StartUpdateRequest) Validate() error {
	if _, err := semver.StrictNewVersion(request.Version); err != nil {
		return httpapi.NewValidationError(map[string]string{"version": "invalid"})
	}
	return nil
}

// UpdaterHandler is the updater's internal API. Only core holds the secret; the gateway has no route here (rule 84).
type UpdaterHandler struct {
	service *UpdaterService
	secret  []byte
	mux     *http.ServeMux
}

func NewUpdaterHandler(service *UpdaterService, secret string) *UpdaterHandler {
	handler := &UpdaterHandler{service: service, secret: []byte(secret), mux: http.NewServeMux()}
	handler.mux.HandleFunc("GET /v1/status", handler.findStatus)
	handler.mux.HandleFunc("POST /v1/check", handler.checkForUpdates)
	handler.mux.HandleFunc("POST /v1/updates", handler.startUpdate)
	return handler
}

func (handler *UpdaterHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !handler.isAuthorized(r) {
		httpapi.WriteError(w, r, errInvalidUpdaterSecret)
		return
	}
	handler.mux.ServeHTTP(w, r)
}

// isAuthorized compares in constant time and never logs the presented value (rule 67).
func (handler *UpdaterHandler) isAuthorized(r *http.Request) bool {
	presented, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	return found && subtle.ConstantTimeCompare([]byte(presented), handler.secret) == 1
}

func (handler *UpdaterHandler) findStatus(w http.ResponseWriter, r *http.Request) {
	status, err := handler.service.Status()
	if err != nil {
		writeUpdaterError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, status)
}

func (handler *UpdaterHandler) checkForUpdates(w http.ResponseWriter, r *http.Request) {
	status, err := handler.service.Check(r.Context())
	if err != nil {
		writeUpdaterError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, status)
}

func (handler *UpdaterHandler) startUpdate(w http.ResponseWriter, r *http.Request) {
	var request StartUpdateRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	if err := request.Validate(); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	status, err := handler.service.StartUpdate(request.Version)
	if err != nil {
		writeUpdaterError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusAccepted, status)
}

func writeUpdaterError(w http.ResponseWriter, r *http.Request, err error) {
	switch code := ErrorCode(err); code {
	case CodeUpdateInProgress, CodeVersionMismatch, CodeNoNewerRelease, ReasonDevelopmentInstallation:
		httpapi.WriteError(w, r, httpapi.NewAPIError(http.StatusConflict, code))
	case releases.CodeChannelUnreachable, releases.CodeChannelRateLimited:
		httpapi.WriteError(w, r, httpapi.NewAPIError(http.StatusBadGateway, code))
	default:
		httpapi.WriteError(w, r, err)
	}
}
