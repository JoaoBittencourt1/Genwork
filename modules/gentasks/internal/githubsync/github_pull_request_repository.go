package githubsync

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
	"github.com/jackc/pgx/v5"
)

type GithubPullRequestRepository struct{}

const githubPullRequestColumns = `github_pull_requests.id, github_pull_requests.token, github_pull_requests.repository,
	github_pull_requests.number, github_pull_requests.title, github_pull_requests.state, github_pull_requests.author_login,
	github_pull_requests.author_avatar_url, github_pull_requests.github_updated_at, github_pull_requests.created_at,
	github_pull_requests.updated_at`

// Upsert and UpdateKnown never let a delivery GitHub sent earlier overwrite a later one; an equal time is a redelivery
// and is stored again, which keeps redeliveries idempotent.
func (repository GithubPullRequestRepository) Upsert(ctx context.Context, db platform.DBTX, snapshot PullRequestSnapshot, at time.Time) (SavedPullRequest, error) {
	var id int64
	err := db.QueryRow(ctx, `
		INSERT INTO github_pull_requests (token, repository, number, title, state, github_updated_at, author_login, author_avatar_url,
		                                  created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
		ON CONFLICT (repository, number) DO UPDATE SET
			title = EXCLUDED.title,
			state = EXCLUDED.state,
			github_updated_at = EXCLUDED.github_updated_at,
			author_login = EXCLUDED.author_login,
			author_avatar_url = EXCLUDED.author_avatar_url,
			updated_at = EXCLUDED.updated_at
		WHERE github_pull_requests.github_updated_at IS NULL OR EXCLUDED.github_updated_at >= github_pull_requests.github_updated_at
		RETURNING id`, platform.NewPublicToken(), snapshot.Repository, snapshot.Number, snapshot.Title, snapshot.State,
		snapshot.GithubUpdatedAt, snapshot.Author.Login, snapshot.Author.AvatarURL, at).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return repository.staleSnapshot(ctx, db, snapshot)
	}
	if err != nil {
		return SavedPullRequest{}, fmt.Errorf("upsert pull request: %w", err)
	}
	return SavedPullRequest{ID: id, Current: true}, nil
}

// UpdateKnown stores the snapshot only for a pull request gentasks already has; found is false otherwise.
func (repository GithubPullRequestRepository) UpdateKnown(ctx context.Context, db platform.DBTX, snapshot PullRequestSnapshot, at time.Time) (saved SavedPullRequest, found bool, err error) {
	var id int64
	err = db.QueryRow(ctx, `
		UPDATE github_pull_requests SET title = $3, state = $4, github_updated_at = $5, author_login = $6, author_avatar_url = $7,
		                                updated_at = $8
		WHERE repository = $1 AND number = $2 AND (github_updated_at IS NULL OR $5 >= github_updated_at)
		RETURNING id`, snapshot.Repository, snapshot.Number, snapshot.Title, snapshot.State, snapshot.GithubUpdatedAt,
		snapshot.Author.Login, snapshot.Author.AvatarURL, at).Scan(&id)
	if err == nil {
		return SavedPullRequest{ID: id, Current: true}, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return SavedPullRequest{}, false, fmt.Errorf("update pull request: %w", err)
	}
	stored, found, err := repository.FindByNumber(ctx, db, snapshot.Repository, snapshot.Number)
	return SavedPullRequest{ID: stored.ID}, found, err
}

func (GithubPullRequestRepository) FindByNumber(ctx context.Context, db platform.DBTX, repository string, number int) (GithubPullRequest, bool, error) {
	pullRequest, err := scanGithubPullRequest(db.QueryRow(ctx, `
		SELECT `+githubPullRequestColumns+` FROM github_pull_requests WHERE repository = $1 AND number = $2`, repository, number))
	if errors.Is(err, pgx.ErrNoRows) {
		return GithubPullRequest{}, false, nil
	}
	if err != nil {
		return GithubPullRequest{}, false, fmt.Errorf("find pull request: %w", err)
	}
	return pullRequest, true, nil
}

func (repository GithubPullRequestRepository) staleSnapshot(ctx context.Context, db platform.DBTX, snapshot PullRequestSnapshot) (SavedPullRequest, error) {
	stored, found, err := repository.FindByNumber(ctx, db, snapshot.Repository, snapshot.Number)
	if err != nil {
		return SavedPullRequest{}, err
	}
	if !found {
		return SavedPullRequest{}, fmt.Errorf("upsert pull request %s#%d: neither stored nor updated", snapshot.Repository, snapshot.Number)
	}
	return SavedPullRequest{ID: stored.ID}, nil
}

// scanGithubPullRequest reads githubPullRequestColumns; a migrated row has no author login, so it has no author.
func scanGithubPullRequest(row pgx.Row, extra ...any) (GithubPullRequest, error) {
	var pullRequest GithubPullRequest
	var authorLogin, authorAvatarURL *string
	destinations := append([]any{&pullRequest.ID, &pullRequest.Token, &pullRequest.Repository, &pullRequest.Number, &pullRequest.Title,
		&pullRequest.State, &authorLogin, &authorAvatarURL, &pullRequest.GithubUpdatedAt, &pullRequest.CreatedAt, &pullRequest.UpdatedAt}, extra...)
	if err := row.Scan(destinations...); err != nil {
		return GithubPullRequest{}, err
	}
	if authorLogin != nil {
		pullRequest.Author = &PullRequestAuthor{Login: *authorLogin}
		if authorAvatarURL != nil {
			pullRequest.Author.AvatarURL = *authorAvatarURL
		}
	}
	return pullRequest, nil
}
