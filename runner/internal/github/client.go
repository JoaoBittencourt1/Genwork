// Package github is the runner's GitHub REST client. It runs on the host with the runner's own token, which never enters a
// sandbox: it opens pull requests and posts reviews.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultTimeout = 30 * time.Second

// APIError is GitHub refusing a request.
type APIError struct {
	Status  int
	Message string
}

func (err *APIError) Error() string {
	return fmt.Sprintf("github answered %d: %s", err.Status, err.Message)
}

type Client struct {
	apiURL     string
	token      string
	httpClient *http.Client
}

func NewClient(apiURL, token string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	return &Client{apiURL: strings.TrimRight(apiURL, "/"), token: token, httpClient: httpClient}
}

type PullRequest struct {
	Number int
	URL    string
}

// ReviewComment is a line comment of a review.
type ReviewComment struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Side string `json:"side"`
	Body string `json:"body"`
}

type Review struct {
	URL string
}

func (client *Client) DefaultBranch(ctx context.Context, repository string) (string, error) {
	var body struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := client.call(ctx, http.MethodGet, "/repos/"+repository, nil, &body); err != nil {
		return "", err
	}
	return body.DefaultBranch, nil
}

func (client *Client) CreatePullRequest(ctx context.Context, repository, head, base, title, description string) (PullRequest, error) {
	var created struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
	}
	err := client.call(ctx, http.MethodPost, "/repos/"+repository+"/pulls", map[string]any{"title": title, "body": description, "head": head, "base": base}, &created)
	return PullRequest{Number: created.Number, URL: created.HTMLURL}, err
}

// CreateReview posts a review with the COMMENT event: a bot never approves or requests changes.
func (client *Client) CreateReview(ctx context.Context, repository string, number int, summary string, comments []ReviewComment) (Review, error) {
	for index := range comments {
		if comments[index].Side == "" {
			comments[index].Side = "RIGHT"
		}
	}
	var created struct {
		HTMLURL string `json:"html_url"`
	}
	body := map[string]any{"body": summary, "event": "COMMENT", "comments": comments}
	err := client.call(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/pulls/%d/reviews", repository, number), body, &created)
	return Review{URL: created.HTMLURL}, err
}

func (client *Client) call(ctx context.Context, method, path string, body any, target any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode github request: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, client.apiURL+(&url.URL{Path: path}).String(), reader)
	if err != nil {
		return fmt.Errorf("build github request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+client.token)
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("Content-Type", "application/json")
	response, err := client.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("call github: %w", err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("read github response: %w", err)
	}
	if response.StatusCode >= http.StatusBadRequest {
		var failure struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(payload, &failure)
		return &APIError{Status: response.StatusCode, Message: failure.Message}
	}
	if target == nil {
		return nil
	}
	return json.Unmarshal(payload, target)
}
