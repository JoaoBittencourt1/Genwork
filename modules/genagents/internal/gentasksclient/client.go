// Package gentasksclient is genagents' client for gentasks' public API, reached through the gateway's internal URL
// (rule 54): the task context bots work from and the tasks assigned to a bot.
package gentasksclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

const DefaultTimeout = 5 * time.Second

var (
	ErrTaskNotFound = httpapi.NewAPIError(http.StatusNotFound, "task_not_found")
	ErrUnavailable  = httpapi.NewAPIError(http.StatusServiceUnavailable, "gentasks_unavailable")
)

type Label struct {
	Token string `json:"token"`
	Name  string `json:"name"`
}

type Board struct {
	Token      string `json:"token"`
	Name       string `json:"name"`
	TaskPrefix string `json:"taskPrefix"`
}

type PullRequest struct {
	Repository string  `json:"repository"`
	Number     int     `json:"number"`
	Title      *string `json:"title"`
	State      *string `json:"state"`
	URL        string  `json:"url"`
}

// IsReviewable says whether a bot review covers the pull request: only open and draft ones.
func (pullRequest PullRequest) IsReviewable() bool {
	return pullRequest.State != nil && (*pullRequest.State == "open" || *pullRequest.State == "draft")
}

type TaskContext struct {
	Token         string        `json:"token"`
	Key           string        `json:"key"`
	Title         string        `json:"title"`
	Description   *string       `json:"description"`
	AssigneeToken *string       `json:"assigneeToken"`
	AssigneeKind  *string       `json:"assigneeKind"`
	Labels        []Label       `json:"labels"`
	Board         Board         `json:"board"`
	PullRequests  []PullRequest `json:"pullRequests"`
	Repository    *string       `json:"repository"`
}

type AssignedTask struct {
	Token      string     `json:"token"`
	AssignedAt *time.Time `json:"assignedAt"`
}

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(gatewayURL string, timeout time.Duration) *Client {
	return &Client{baseURL: strings.TrimRight(gatewayURL, "/"), httpClient: &http.Client{Timeout: timeout}}
}

// FindTaskContext reads a task with the caller's own token, so gentasks applies the caller's rights.
func (client *Client) FindTaskContext(ctx context.Context, accessToken, taskToken string) (TaskContext, error) {
	var taskContext TaskContext
	err := client.get(ctx, accessToken, "/api/gentasks/v1/tasks/"+url.PathEscape(taskToken)+"/context", &taskContext)
	return taskContext, err
}

// ListAssignedTasks lists the active tasks assigned to whoever the token belongs to.
func (client *Client) ListAssignedTasks(ctx context.Context, accessToken string, limit int) ([]AssignedTask, error) {
	var body struct {
		Tasks []AssignedTask `json:"tasks"`
	}
	err := client.get(ctx, accessToken, "/api/gentasks/v1/me/assigned-tasks?limit="+strconv.Itoa(limit), &body)
	return body.Tasks, err
}

func (client *Client) get(ctx context.Context, accessToken, path string, target any) error {
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("build gentasks request: %w", err)
	}
	httpRequest.Header.Set("Authorization", "Bearer "+accessToken)
	response, err := client.httpClient.Do(httpRequest)
	if err != nil {
		return errors.Join(ErrUnavailable, err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return errors.Join(ErrUnavailable, err)
	}
	switch {
	case response.StatusCode == http.StatusNotFound:
		return ErrTaskNotFound
	case response.StatusCode == http.StatusForbidden:
		return httpapi.ErrForbidden
	case response.StatusCode == http.StatusUnauthorized:
		return httpapi.ErrInvalidToken
	case response.StatusCode != http.StatusOK:
		return ErrUnavailable
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("decode gentasks response: %w", err)
	}
	return nil
}
