package githubsync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const githubInstallationTimeout = 10 * time.Second

var (
	ErrAppNotInstalled      = errors.New("the GitHub App is not installed on the repository")
	ErrAppPermissionsDenied = errors.New("GitHub refused the permissions the token asks for")
)

// InstallationToken is a GitHub installation access token; it lives an hour and is never stored.
type InstallationToken struct {
	Token     GithubCredential
	ExpiresAt time.Time
}

// GithubInstallationClient calls the GitHub App endpoints that hand out installation tokens.
type GithubInstallationClient struct {
	apiURL     string
	httpClient *http.Client
}

func NewGithubInstallationClient(apiURL string) *GithubInstallationClient {
	return &GithubInstallationClient{apiURL: strings.TrimRight(apiURL, "/"), httpClient: &http.Client{Timeout: githubInstallationTimeout}}
}

// FindRepositoryInstallation returns the id of the App's installation that covers the repository.
func (client *GithubInstallationClient) FindRepositoryInstallation(ctx context.Context, appJWT, repository string) (int64, error) {
	var installation struct {
		ID int64 `json:"id"`
	}
	status, err := client.call(ctx, http.MethodGet, "/repos/"+repository+"/installation", appJWT, nil, &installation)
	switch {
	case status == http.StatusNotFound:
		return 0, ErrAppNotInstalled
	case err != nil:
		return 0, err
	}
	return installation.ID, nil
}

// CreateInstallationToken asks for a token limited to the named repositories and permissions.
func (client *GithubInstallationClient) CreateInstallationToken(ctx context.Context, appJWT string, installationID int64, repositoryNames []string, permissions map[string]string) (InstallationToken, error) {
	var created struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	body := map[string]any{"repositories": repositoryNames, "permissions": permissions}
	status, err := client.call(ctx, http.MethodPost, "/app/installations/"+strconv.FormatInt(installationID, 10)+"/access_tokens", appJWT, body, &created)
	switch {
	case status == http.StatusUnprocessableEntity || status == http.StatusForbidden:
		return InstallationToken{}, ErrAppPermissionsDenied
	case status == http.StatusNotFound:
		return InstallationToken{}, ErrAppNotInstalled
	case err != nil:
		return InstallationToken{}, err
	}
	if created.Token == "" {
		return InstallationToken{}, fmt.Errorf("%w: no token in the answer", ErrGithubUnavailable)
	}
	return InstallationToken{Token: NewGithubCredential([]byte(created.Token)), ExpiresAt: created.ExpiresAt}, nil
}

func (client *GithubInstallationClient) call(ctx context.Context, method, path, appJWT string, body, target any) (int, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return 0, fmt.Errorf("encode github request: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, client.apiURL+path, reader)
	if err != nil {
		return 0, fmt.Errorf("build github request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
	request.Header.Set("Authorization", "Bearer "+appJWT)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrGithubUnavailable, err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, maxConversionBodyBytes))
	if err != nil {
		return response.StatusCode, fmt.Errorf("%w: %v", ErrGithubUnavailable, err)
	}
	if response.StatusCode >= http.StatusBadRequest {
		return response.StatusCode, fmt.Errorf("%w: status %d", ErrGithubUnavailable, response.StatusCode)
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return response.StatusCode, fmt.Errorf("%w: decode: %v", ErrGithubUnavailable, err)
	}
	return response.StatusCode, nil
}
