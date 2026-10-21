package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const DefaultRequestTimeout = 15 * time.Second

const maxResponseBytes = 4 << 20

var ErrInstanceUnavailable = errors.New("the Genwork instance is unavailable")

// APIError is an error response of the Genwork API: {"error": {"code": ..., "fields": {...}}}.
type APIError struct {
	Status int
	Code   string
	Fields map[string]string
}

func (e *APIError) Error() string {
	message := fmt.Sprintf("genwork API answered %d %s", e.Status, e.Code)
	if len(e.Fields) == 0 {
		return message
	}
	names := make([]string, 0, len(e.Fields))
	for name := range e.Fields {
		names = append(names, name)
	}
	sort.Strings(names)
	problems := make([]string, 0, len(names))
	for _, name := range names {
		problems = append(problems, name+": "+e.Fields[name])
	}
	return message + " (" + strings.Join(problems, ", ") + ")"
}

func IsAPIStatus(err error, status int) bool {
	var apiError *APIError
	return errors.As(err, &apiError) && apiError.Status == status
}

type APIRequest struct {
	Method string
	Path   string
	Query  url.Values
	Body   any
}

// ExpandPath fills each %s of the pattern with a path-escaped token, so a token can never change the route.
func ExpandPath(pattern string, tokens ...string) string {
	escaped := make([]any, 0, len(tokens))
	for _, token := range tokens {
		escaped = append(escaped, url.PathEscape(token))
	}
	return fmt.Sprintf(pattern, escaped...)
}

type APIRequester interface {
	Send(ctx context.Context, request APIRequest) (json.RawMessage, error)
}

type errorResponseBody struct {
	Error struct {
		Code   string            `json:"code"`
		Fields map[string]string `json:"fields"`
	} `json:"error"`
}

// APIClient sends requests to the instance gateway; it answers the raw JSON body, or nil for an empty one.
type APIClient struct {
	instanceURL string
	httpClient  *http.Client
}

func NewAPIClient(instanceURL string, httpClient *http.Client) *APIClient {
	return &APIClient{instanceURL: strings.TrimRight(instanceURL, "/"), httpClient: httpClient}
}

func (client *APIClient) SendWithToken(ctx context.Context, request APIRequest, accessToken string) (json.RawMessage, error) {
	httpRequest, err := client.buildHTTPRequest(ctx, request)
	if err != nil {
		return nil, err
	}
	if accessToken != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+accessToken)
	}
	response, err := client.httpClient.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInstanceUnavailable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("%w: read response: %v", ErrInstanceUnavailable, err)
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return nil, newAPIError(response.StatusCode, body)
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, nil
	}
	return json.RawMessage(body), nil
}

func (client *APIClient) buildHTTPRequest(ctx context.Context, request APIRequest) (*http.Request, error) {
	endpoint := client.instanceURL + request.Path
	if len(request.Query) > 0 {
		endpoint += "?" + request.Query.Encode()
	}
	var body io.Reader
	if request.Body != nil {
		encoded, err := json.Marshal(request.Body)
		if err != nil {
			return nil, fmt.Errorf("encode request body: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, request.Method, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpRequest.Header.Set("Accept", "application/json")
	if request.Body != nil {
		httpRequest.Header.Set("Content-Type", "application/json")
	}
	return httpRequest, nil
}

func newAPIError(status int, body []byte) *APIError {
	var decoded errorResponseBody
	if err := json.Unmarshal(body, &decoded); err != nil || decoded.Error.Code == "" {
		return &APIError{Status: status, Code: strings.ReplaceAll(strings.ToLower(http.StatusText(status)), " ", "_")}
	}
	return &APIError{Status: status, Code: decoded.Error.Code, Fields: decoded.Error.Fields}
}
