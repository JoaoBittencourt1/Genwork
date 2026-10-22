package githubsync

import (
	"errors"
	"io"
	"net/http"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

const (
	maxWebhookBodyBytes    = 5 << 20
	githubEventHeader      = "X-GitHub-Event"
	githubSignatureHeader  = "X-Hub-Signature-256"
	githubEventPullRequest = "pull_request"
)

var (
	ErrInvalidWebhookSignature = httpapi.NewAPIError(http.StatusUnauthorized, "invalid_signature")
	ErrWebhookPayloadTooLarge  = httpapi.NewAPIError(http.StatusRequestEntityTooLarge, "payload_too_large")
)

type GithubWebhookHandler struct {
	connections *GithubConnectionService
	sync        *PullRequestSyncService
}

func NewGithubWebhookHandler(connections *GithubConnectionService, sync *PullRequestSyncService) *GithubWebhookHandler {
	return &GithubWebhookHandler{connections: connections, sync: sync}
}

// Register: the webhook carries no access token; its authorization rule is the signature, checked before the body is parsed (rule 18).
func (handler *GithubWebhookHandler) Register(router *httpapi.Router) {
	router.HandleFunc("POST "+WebhookPath, httpapi.Public, handler.receiveDelivery)
}

func (handler *GithubWebhookHandler) receiveDelivery(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBodyBytes))
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		httpapi.WriteError(w, r, ErrWebhookPayloadTooLarge)
		return
	}
	if err != nil {
		httpapi.WriteError(w, r, httpapi.ErrMalformedBody)
		return
	}
	if err := handler.connections.VerifyDelivery(r.Context(), body, r.Header.Get(githubSignatureHeader)); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	if r.Header.Get(githubEventHeader) != githubEventPullRequest {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	event, err := DecodePullRequestEvent(body)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	if err := handler.sync.SyncPullRequest(r.Context(), event); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
