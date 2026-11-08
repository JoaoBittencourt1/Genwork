package github_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

func TestRightSideLinesAreTheAddedAndContextLinesOfEachHunk(t *testing.T) {
	patch := "@@ -10,4 +10,5 @@ func a() {\n context\n-removed\n+added\n+added too\n context\n\\ No newline at end of file\n@@ -40 +41,2 @@\n+x\n y"

	lines := github.RightSideLines(patch)

	for _, want := range []int{10, 11, 12, 13, 41, 42} {
		if !lines[want] {
			t.Errorf("line %d is missing: %v", want, lines)
		}
	}
	if len(lines) != 6 || lines[14] || lines[40] {
		t.Errorf("lines = %v", lines)
	}
	if len(github.RightSideLines("")) != 0 {
		t.Error("an empty patch has lines")
	}
}

func TestCommentableLinesReadsEveryPageOfFiles(t *testing.T) {
	var pages []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pages = append(pages, r.URL.Query().Get("page"))
		files := []map[string]string{}
		if r.URL.Query().Get("page") == "1" {
			for index := 0; index < 100; index++ {
				files = append(files, map[string]string{"filename": fmt.Sprintf("f%d.go", index), "patch": "@@ -1 +1 @@\n+a"})
			}
		} else {
			files = append(files, map[string]string{"filename": "last.go", "patch": "@@ -0,0 +7,2 @@\n+a\n+b"}, map[string]string{"filename": "image.png"})
		}
		_ = json.NewEncoder(w).Encode(files)
	}))
	t.Cleanup(server.Close)

	lines, err := github.NewClient(server.URL, "ghs-token", nil).CommentableLines(context.Background(), "acme/api", 12)

	if err != nil || len(pages) != 2 || len(lines) != 102 || !lines["last.go"][8] || !lines["f0.go"][1] || len(lines["image.png"]) != 0 {
		t.Errorf("pages = %v, files = %d, err = %v", pages, len(lines), err)
	}
}

func TestAnUnprocessableReviewSaysWhatGithubRefused(t *testing.T) {
	client, _ := newServer(t, 422, `{"message":"Unprocessable Entity","errors":["Line could not be resolved"]}`)

	_, err := client.CreateReview(context.Background(), "acme/api", 12, "summary", []github.ReviewComment{{Path: "a.go", Line: 900, Body: "x"}})

	var apiErr *github.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 422 || apiErr.Message != "Unprocessable Entity: Line could not be resolved" {
		t.Errorf("err = %v", err)
	}
}
