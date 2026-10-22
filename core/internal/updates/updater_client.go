package updates

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	maxUpdaterResponseBytes = 1 << 20
	// The updater gives the release channel 15 seconds for a check; core waits a little longer for its answer.
	updaterCheckTimeout = 20 * time.Second
	updaterTimeout      = 10 * time.Second
)

// UpdaterClient is the boundary to the updater container (rule 9); tests replace it with a recorded fake.
type UpdaterClient interface {
	FindStatus(ctx context.Context) (UpdaterStatus, error)
	CheckForUpdates(ctx context.Context) (UpdaterStatus, error)
	StartUpdate(ctx context.Context, version string) (UpdaterStatus, error)
}

// UpdaterRefusal is an answer of the updater that is not a status, with its error code.
type UpdaterRefusal struct {
	Status int
	Code   string
}

func (refusal *UpdaterRefusal) Error() string {
	return fmt.Sprintf("the updater answered %d %s", refusal.Status, refusal.Code)
}

var errUpdaterUnreachable = errors.New("the updater cannot be reached")

type HTTPUpdaterClient struct {
	baseURL string
	secret  string
	client  *http.Client
}

func NewHTTPUpdaterClient(baseURL, secret string) *HTTPUpdaterClient {
	return &HTTPUpdaterClient{baseURL: baseURL, secret: secret, client: &http.Client{}}
}

func (updater *HTTPUpdaterClient) FindStatus(ctx context.Context) (UpdaterStatus, error) {
	return updater.call(ctx, http.MethodGet, "/v1/status", nil, updaterTimeout)
}

func (updater *HTTPUpdaterClient) CheckForUpdates(ctx context.Context) (UpdaterStatus, error) {
	return updater.call(ctx, http.MethodPost, "/v1/check", nil, updaterCheckTimeout)
}

func (updater *HTTPUpdaterClient) StartUpdate(ctx context.Context, version string) (UpdaterStatus, error) {
	return updater.call(ctx, http.MethodPost, "/v1/updates", map[string]string{"version": version}, updaterTimeout)
}

// call sends the shared secret, which never appears in errors or logs (rule 67).
func (updater *HTTPUpdaterClient) call(ctx context.Context, method, path string, body any, timeout time.Duration) (UpdaterStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return UpdaterStatus{}, err
		}
		payload = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, updater.baseURL+path, payload)
	if err != nil {
		return UpdaterStatus{}, err
	}
	request.Header.Set("Authorization", "Bearer "+updater.secret)
	request.Header.Set("Content-Type", "application/json")
	response, err := updater.client.Do(request)
	if err != nil {
		return UpdaterStatus{}, errUpdaterUnreachable
	}
	defer response.Body.Close()
	reader := io.LimitReader(response.Body, maxUpdaterResponseBytes)
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusAccepted {
		var refusal struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		_ = json.NewDecoder(reader).Decode(&refusal)
		return UpdaterStatus{}, &UpdaterRefusal{Status: response.StatusCode, Code: refusal.Error.Code}
	}
	var status UpdaterStatus
	if err := json.NewDecoder(reader).Decode(&status); err != nil {
		return UpdaterStatus{}, fmt.Errorf("decode the updater status: %w", err)
	}
	return status, nil
}
