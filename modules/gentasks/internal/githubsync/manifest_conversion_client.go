package githubsync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	githubConversionTimeout = 10 * time.Second
	githubAPIVersion        = "2022-11-28"
	maxConversionBodyBytes  = 1 << 20
)

var (
	ErrManifestCodeRejected = errors.New("github rejected the manifest code")
	ErrGithubUnavailable    = errors.New("github is unavailable")
)

const githubOwnerTypeOrganization = "Organization"

// ConvertedGithubApp holds what gentasks keeps from a manifest conversion; the OAuth client secret is never decoded.
type ConvertedGithubApp struct {
	GithubAppID         int64
	Slug                string
	OwnerLogin          string
	OwnerIsOrganization bool
	WebhookSecret       GithubCredential
	PrivateKey          GithubCredential
}

type manifestConversionBody struct {
	ID    int64  `json:"id"`
	Slug  string `json:"slug"`
	Owner struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	} `json:"owner"`
	WebhookSecret *string `json:"webhook_secret"`
	PEM           string  `json:"pem"`
}

// GithubManifestConversionClient exchanges the one-time code GitHub returns at the end of the manifest flow.
type GithubManifestConversionClient struct {
	apiURL     string
	httpClient *http.Client
}

func NewGithubManifestConversionClient(apiURL string) *GithubManifestConversionClient {
	return &GithubManifestConversionClient{apiURL: strings.TrimRight(apiURL, "/"), httpClient: &http.Client{Timeout: githubConversionTimeout}}
}

func (client *GithubManifestConversionClient) ConvertManifest(ctx context.Context, code string) (ConvertedGithubApp, error) {
	endpoint := client.apiURL + "/app-manifests/" + url.PathEscape(code) + "/conversions"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return ConvertedGithubApp{}, fmt.Errorf("build manifest conversion request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
	response, err := client.httpClient.Do(request)
	if err != nil {
		return ConvertedGithubApp{}, fmt.Errorf("%w: %v", ErrGithubUnavailable, err)
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusCreated:
		return decodeConvertedGithubApp(io.LimitReader(response.Body, maxConversionBodyBytes))
	case http.StatusNotFound, http.StatusUnprocessableEntity:
		return ConvertedGithubApp{}, ErrManifestCodeRejected
	default:
		return ConvertedGithubApp{}, fmt.Errorf("%w: status %d", ErrGithubUnavailable, response.StatusCode)
	}
}

func decodeConvertedGithubApp(body io.Reader) (ConvertedGithubApp, error) {
	var decoded manifestConversionBody
	if err := json.NewDecoder(body).Decode(&decoded); err != nil {
		return ConvertedGithubApp{}, fmt.Errorf("%w: decode manifest conversion: %v", ErrGithubUnavailable, err)
	}
	if decoded.ID == 0 || decoded.Slug == "" || decoded.Owner.Login == "" || decoded.WebhookSecret == nil || *decoded.WebhookSecret == "" || decoded.PEM == "" {
		return ConvertedGithubApp{}, fmt.Errorf("%w: manifest conversion is missing fields", ErrGithubUnavailable)
	}
	return ConvertedGithubApp{
		GithubAppID:         decoded.ID,
		Slug:                decoded.Slug,
		OwnerLogin:          decoded.Owner.Login,
		OwnerIsOrganization: decoded.Owner.Type == githubOwnerTypeOrganization,
		WebhookSecret:       NewGithubCredential([]byte(*decoded.WebhookSecret)),
		PrivateKey:          NewGithubCredential([]byte(decoded.PEM)),
	}, nil
}
