package execution

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/github"
)

// GithubAdapter lets the executor use the REST client through its own small interface.
type GithubAdapter struct {
	client *github.Client
}

func NewGithubAdapter(client *github.Client) GithubAdapter {
	return GithubAdapter{client: client}
}

func (adapter GithubAdapter) WithToken(token string) GithubPoster {
	return GithubAdapter{client: adapter.client.WithToken(token)}
}

func (adapter GithubAdapter) DefaultBranch(ctx context.Context, repository string) (string, error) {
	return adapter.client.DefaultBranch(ctx, repository)
}

func (adapter GithubAdapter) CreatePullRequest(ctx context.Context, repository, head, base, title, description string) (PullRequestCreated, error) {
	created, err := adapter.client.CreatePullRequest(ctx, repository, head, base, title, description)
	return PullRequestCreated{Number: created.Number, URL: created.URL}, err
}

func (adapter GithubAdapter) CreateReview(ctx context.Context, repository string, number int, summary string, comments []ReviewComment) (string, error) {
	converted := make([]github.ReviewComment, 0, len(comments))
	for _, comment := range comments {
		converted = append(converted, github.ReviewComment{Path: comment.Path, Line: comment.Line, Body: comment.Body})
	}
	review, err := adapter.client.CreateReview(ctx, repository, number, summary, converted)
	var apiErr *github.APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusUnprocessableEntity {
		return "", fmt.Errorf("%w: %v", ErrReviewContentRejected, err)
	}
	return review.URL, err
}

func (adapter GithubAdapter) CommentableLines(ctx context.Context, repository string, number int) (map[string]map[int]bool, error) {
	return adapter.client.CommentableLines(ctx, repository, number)
}
