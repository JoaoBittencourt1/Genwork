// Package api is the runner's client for genagents' runner API (/api/genagents/v1/runner/...), authenticated with the
// runner secret.
package api

import (
	"bytes"
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
	basePath = "/api/genagents/v1/runner"
	// ClaimTimeout is longer than the server's 25 second long poll.
	ClaimTimeout   = 40 * time.Second
	DefaultTimeout = 15 * time.Second
)

// StatusError is a refusal from genagents.
type StatusError struct {
	Status int
	Code   string
}

func (err *StatusError) Error() string {
	return fmt.Sprintf("genagents answered %d %s", err.Status, err.Code)
}

// IsRevoked tells the runner's secret was refused: nothing it holds is worth keeping.
func (err *StatusError) IsRevoked() bool {
	return err.Status == http.StatusUnauthorized
}

type Client struct {
	baseURL    string
	secret     string
	httpClient *http.Client
}

func NewClient(instanceURL, secret string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: DefaultTimeout}
	}
	return &Client{baseURL: strings.TrimRight(instanceURL, "/") + basePath, secret: secret, httpClient: httpClient}
}

type Heartbeat struct {
	ContainerEngine      string            `json:"containerEngine"`
	SandboxImage         string            `json:"sandboxImage"`
	CustomImage          bool              `json:"customImage"`
	DirectoryMode        bool              `json:"directoryMode"`
	Tools                map[string]string `json:"tools"`
	HasGithubCredentials bool              `json:"hasGithubCredentials"`
	MaxConcurrency       int               `json:"maxConcurrency"`
}

type AccountAssignment struct {
	Token    string `json:"token"`
	Provider string `json:"provider"`
}

type HeartbeatAnswer struct {
	Accounts []AccountAssignment `json:"accounts"`
}

type ClaimedAccount struct {
	Token    string `json:"token"`
	Provider string `json:"provider"`
	Mode     string `json:"mode"`
	APIKey   string `json:"apiKey"`
}

type SkillFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type Skill struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Body        string      `json:"body"`
	Version     int         `json:"version"`
	Files       []SkillFile `json:"files"`
}

type Entry struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type MCPServer struct {
	Name      string   `json:"name"`
	Transport string   `json:"transport"`
	URL       string   `json:"url"`
	Headers   []Entry  `json:"headers"`
	Command   string   `json:"command"`
	Args      []string `json:"args"`
	Env       []Entry  `json:"env"`
}

type PullRequestRef struct {
	Repository string `json:"repository"`
	Number     int    `json:"number"`
	URL        string `json:"url"`
	Title      string `json:"title"`
}

type Bot struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// Run is the specification a claim returns: everything needed to execute, secrets included.
type Run struct {
	RunToken       string           `json:"runToken"`
	Kind           string           `json:"kind"`
	Bot            Bot              `json:"bot"`
	Prompt         string           `json:"prompt"`
	Instructions   string           `json:"instructions"`
	Model          string           `json:"model"`
	Effort         string           `json:"effort"`
	MaxTurns       int              `json:"maxTurns"`
	TimeoutMinutes int              `json:"timeoutMinutes"`
	Account        ClaimedAccount   `json:"account"`
	Skills         []Skill          `json:"skills"`
	MCPServers     []MCPServer      `json:"mcpServers"`
	GenworkMCPURL  string           `json:"genworkMcpUrl"`
	ResumeSession  *string          `json:"resumeSession"`
	Repository     string           `json:"repository"`
	Branch         string           `json:"branch"`
	TaskKey        string           `json:"taskKey"`
	TaskTitle      string           `json:"taskTitle"`
	PullRequests   []PullRequestRef `json:"pullRequests"`
}

type Event struct {
	Sequence int64           `json:"sequence"`
	Type     string          `json:"type"`
	Payload  json.RawMessage `json:"payload"`
}

type Message struct {
	Sequence int64  `json:"sequence"`
	Text     string `json:"text"`
}

type GenworkToken struct {
	AccessToken string    `json:"accessToken"`
	ExpiresAt   time.Time `json:"expiresAt"`
	MCPURL      string    `json:"mcpUrl"`
}

// Register trades a registration token for the runner's secret; it needs no secret yet.
func Register(ctx context.Context, instanceURL, registrationToken string, httpClient *http.Client) (token, secret string, err error) {
	client := NewClient(instanceURL, "", httpClient)
	var answer struct {
		RunnerToken string `json:"runnerToken"`
		Secret      string `json:"secret"`
	}
	err = client.call(ctx, http.MethodPost, "/register", map[string]any{"registrationToken": registrationToken}, &answer, false)
	return answer.RunnerToken, answer.Secret, err
}

func (client *Client) SendHeartbeat(ctx context.Context, heartbeat Heartbeat) (HeartbeatAnswer, error) {
	var answer HeartbeatAnswer
	err := client.call(ctx, http.MethodPost, "/heartbeat", heartbeat, &answer, true)
	return answer, err
}

// Claim long-polls for one run; it returns nil when none came.
func (client *Client) Claim(ctx context.Context, freeSlots int) (*Run, error) {
	var run Run
	found, err := client.callOptional(ctx, http.MethodPost, "/claims", map[string]any{"freeSlots": freeSlots}, &run, ClaimTimeout)
	if err != nil || !found {
		return nil, err
	}
	return &run, nil
}

func (client *Client) PostEvents(ctx context.Context, runToken string, events []Event) (state string, err error) {
	var answer struct {
		State string `json:"state"`
	}
	err = client.call(ctx, http.MethodPost, "/runs/"+url.PathEscape(runToken)+"/events", map[string]any{"events": events}, &answer, true)
	return answer.State, err
}

func (client *Client) RunState(ctx context.Context, runToken string) (string, error) {
	var answer struct {
		State string `json:"state"`
	}
	err := client.call(ctx, http.MethodGet, "/runs/"+url.PathEscape(runToken), nil, &answer, true)
	return answer.State, err
}

func (client *Client) ListMessages(ctx context.Context, runToken string) ([]Message, error) {
	var answer struct {
		Messages []Message `json:"messages"`
	}
	err := client.call(ctx, http.MethodGet, "/runs/"+url.PathEscape(runToken)+"/messages", nil, &answer, true)
	return answer.Messages, err
}

func (client *Client) AcknowledgeMessages(ctx context.Context, runToken string, upToSequence int64) error {
	return client.call(ctx, http.MethodPost, "/runs/"+url.PathEscape(runToken)+"/messages/ack", map[string]any{"upToSequence": upToSequence}, nil, true)
}

func (client *Client) GenworkToken(ctx context.Context, runToken string) (GenworkToken, error) {
	var token GenworkToken
	err := client.call(ctx, http.MethodPost, "/runs/"+url.PathEscape(runToken)+"/genwork-token", nil, &token, true)
	return token, err
}

func (client *Client) ReportAccountStatus(ctx context.Context, accountToken string, signedIn bool, message string) error {
	return client.call(ctx, http.MethodPost, "/accounts/"+url.PathEscape(accountToken)+"/status", map[string]any{"signedIn": signedIn, "message": message}, nil, true)
}

func (client *Client) call(ctx context.Context, method, path string, body any, target any, authenticated bool) error {
	found, err := client.send(ctx, method, path, body, target, authenticated, 0)
	if err == nil && !found && target != nil {
		return errors.New("genagents answered without a body")
	}
	return err
}

func (client *Client) callOptional(ctx context.Context, method, path string, body any, target any, timeout time.Duration) (bool, error) {
	return client.send(ctx, method, path, body, target, true, timeout)
}

func (client *Client) send(ctx context.Context, method, path string, body any, target any, authenticated bool, timeout time.Duration) (bool, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return false, fmt.Errorf("encode request: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	httpRequest, err := http.NewRequestWithContext(ctx, method, client.baseURL+path, reader)
	if err != nil {
		return false, fmt.Errorf("build request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	if authenticated {
		httpRequest.Header.Set("Authorization", "Bearer "+client.secret)
	}
	httpClient := client.httpClient
	if timeout > 0 {
		httpClient = &http.Client{Transport: client.httpClient.Transport}
	}
	response, err := httpClient.Do(httpRequest)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil {
		return false, err
	}
	if response.StatusCode == http.StatusNoContent || (response.StatusCode == http.StatusOK && len(payload) == 0) {
		return false, nil
	}
	if response.StatusCode >= http.StatusBadRequest {
		return false, &StatusError{Status: response.StatusCode, Code: decodeCode(payload)}
	}
	if target == nil {
		return true, nil
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return false, fmt.Errorf("decode response: %w", err)
	}
	return true, nil
}

func decodeCode(payload []byte) string {
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(payload, &body) != nil || body.Error.Code == "" {
		return "unknown"
	}
	return body.Error.Code
}
