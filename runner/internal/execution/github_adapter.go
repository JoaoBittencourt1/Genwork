package execution

import (
	"context"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/github"
)

// GithubAdapter lets the executor use the REST client through its own small interface.
type GithubAdapter struct {
	client *github.Client
}

func NewGithubAdapter(client *github.Client) GithubAdapter {
	return GithubAdapter{client: client}
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
	return review.URL, err
}
