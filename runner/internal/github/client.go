// Package github is the runner's GitHub REST client. It runs on the host with the run's token from the platform or the
// runner's own token, neither of which ever enters a sandbox: it opens pull requests and posts reviews.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
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

// WithToken is the same client authenticating with another token, such as a run's own.
func (client *Client) WithToken(token string) *Client {
	return &Client{apiURL: client.apiURL, token: token, httpClient: client.httpClient}
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

// maxPullRequestFilePages bounds the files read from a pull request: GitHub lists at most 3000 files, 100 per page.
const maxPullRequestFilePages = 30

// CommentableLines maps each changed file of a pull request to the lines of its new version a review can comment on:
// the added and context lines of its diff hunks. A file without a patch (binary, or too large) has none.
func (client *Client) CommentableLines(ctx context.Context, repository string, number int) (map[string]map[int]bool, error) {
	lines := map[string]map[int]bool{}
	for page := 1; page <= maxPullRequestFilePages; page++ {
		var files []struct {
			Filename string `json:"filename"`
			Patch    string `json:"patch"`
		}
		path := fmt.Sprintf("/repos/%s/pulls/%d/files?per_page=100&page=%d", repository, number, page)
		if err := client.call(ctx, http.MethodGet, path, nil, &files); err != nil {
			return nil, err
		}
		for _, file := range files {
			lines[file.Filename] = RightSideLines(file.Patch)
		}
		if len(files) < 100 {
			break
		}
	}
	return lines, nil
}

// RightSideLines reads a unified diff patch and returns the line numbers of the new file that appear in it.
func RightSideLines(patch string) map[int]bool {
	lines := map[int]bool{}
	current := 0
	for _, line := range strings.Split(patch, "\n") {
		switch {
		case strings.HasPrefix(line, "@@"):
			current = hunkStart(line)
		case current == 0, strings.HasPrefix(line, "\\"):
		case strings.HasPrefix(line, "-"):
		default:
			lines[current] = true
			current++
		}
	}
	return lines
}

// hunkStart reads the new file's first line from a hunk header such as "@@ -10,4 +12,6 @@".
func hunkStart(header string) int {
	_, after, found := strings.Cut(header, "+")
	if !found {
		return 0
	}
	digits := after
	if end := strings.IndexAny(digits, ", @"); end >= 0 {
		digits = digits[:end]
	}
	start, err := strconv.Atoi(digits)
	if err != nil {
		return 0
	}
	return start
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

// describeFailure keeps GitHub's message and the details in its errors list, which say what was refused; a 422 message
// alone is just "Unprocessable Entity".
func describeFailure(payload []byte) string {
	var failure struct {
		Message string            `json:"message"`
		Errors  []json.RawMessage `json:"errors"`
	}
	_ = json.Unmarshal(payload, &failure)
	details := make([]string, 0, len(failure.Errors))
	for _, raw := range failure.Errors {
		var text string
		if json.Unmarshal(raw, &text) == nil && text != "" {
			details = append(details, text)
			continue
		}
		var detail struct {
			Message string `json:"message"`
			Field   string `json:"field"`
			Code    string `json:"code"`
		}
		if json.Unmarshal(raw, &detail) == nil {
			details = append(details, strings.TrimSpace(strings.Join(nonEmpty(detail.Message, detail.Field, detail.Code), " ")))
		}
	}
	if len(details) == 0 {
		return failure.Message
	}
	return failure.Message + ": " + strings.Join(details, "; ")
}

func nonEmpty(values ...string) []string {
	kept := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			kept = append(kept, value)
		}
	}
	return kept
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
	pathOnly, query, _ := strings.Cut(path, "?")
	request, err := http.NewRequestWithContext(ctx, method, client.apiURL+(&url.URL{Path: pathOnly, RawQuery: query}).String(), reader)
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
		return &APIError{Status: response.StatusCode, Message: describeFailure(payload)}
	}
	if target == nil {
		return nil
	}
	return json.Unmarshal(payload, target)
}
