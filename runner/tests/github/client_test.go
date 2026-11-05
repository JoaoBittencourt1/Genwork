package github_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/github"
)

type seenRequest struct {
	Method string
	Path   string
	Auth   string
	Body   map[string]any
}

func newServer(t *testing.T, status int, answer string) (*github.Client, *[]seenRequest) {
	t.Helper()
	var seen []seenRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(payload, &body)
		seen = append(seen, seenRequest{Method: r.Method, Path: r.URL.Path, Auth: r.Header.Get("Authorization"), Body: body})
		w.WriteHeader(status)
		_, _ = w.Write([]byte(answer))
	}))
	t.Cleanup(server.Close)
	return github.NewClient(server.URL+"/", "ghp-token", nil), &seen
}

func TestDefaultBranch(t *testing.T) {
	client, seen := newServer(t, 200, `{"default_branch":"develop"}`)

	branch, err := client.DefaultBranch(context.Background(), "acme/api")

	if err != nil || branch != "develop" || (*seen)[0].Path != "/repos/acme/api" || (*seen)[0].Auth != "Bearer ghp-token" {
		t.Errorf("branch = %q, err = %v, seen = %+v", branch, err, *seen)
	}
}

func TestCreatePullRequest(t *testing.T) {
	client, seen := newServer(t, 201, `{"number":12,"html_url":"https://github.com/acme/api/pull/12"}`)

	created, err := client.CreatePullRequest(context.Background(), "acme/api", "bot/gen-1-abc", "main", "GEN-1 Speed up", "Faster")

	if err != nil || created.Number != 12 || created.URL != "https://github.com/acme/api/pull/12" {
		t.Fatalf("created = %+v, err = %v", created, err)
	}
	request := (*seen)[0]
	if request.Method != "POST" || request.Path != "/repos/acme/api/pulls" || request.Body["head"] != "bot/gen-1-abc" || request.Body["base"] != "main" || request.Body["title"] != "GEN-1 Speed up" {
		t.Errorf("request = %+v", request)
	}
}

func TestReviewsUseTheCommentEventAndRightSideComments(t *testing.T) {
	client, seen := newServer(t, 200, `{"html_url":"https://github.com/acme/api/pull/12#pullrequestreview-1"}`)

	review, err := client.CreateReview(context.Background(), "acme/api", 12, "Two issues", []github.ReviewComment{{Path: "a.go", Line: 3, Body: "nil check"}})

	if err != nil || review.URL == "" {
		t.Fatalf("review = %+v, err = %v", review, err)
	}
	request := (*seen)[0]
	comments := request.Body["comments"].([]any)
	if request.Path != "/repos/acme/api/pulls/12/reviews" || request.Body["event"] != "COMMENT" || request.Body["body"] != "Two issues" ||
		len(comments) != 1 || comments[0].(map[string]any)["side"] != "RIGHT" || comments[0].(map[string]any)["line"].(float64) != 3 {
		t.Errorf("request = %+v", request)
	}
}

func TestRefusalsKeepStatusAndMessage(t *testing.T) {
	client, _ := newServer(t, 422, `{"message":"Validation Failed"}`)

	_, err := client.CreatePullRequest(context.Background(), "acme/api", "h", "b", "t", "d")

	var apiErr *github.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 422 || apiErr.Message != "Validation Failed" {
		t.Errorf("error = %v", err)
	}
	if _, err := client.CreateReview(context.Background(), "acme/api", 1, "s", nil); err == nil {
		t.Error("a refusal was swallowed")
	}
}
