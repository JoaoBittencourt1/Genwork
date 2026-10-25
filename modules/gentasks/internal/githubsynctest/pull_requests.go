package githubsynctest

import (
	"context"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/boardtest"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsync"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
)

const (
	PullRequestRepository      = "acme/api"
	PullRequestAuthorLogin     = "ana-dev"
	PullRequestAuthorAvatarURL = "https://avatars.githubusercontent.com/u/5001?v=4"
)

// PullRequestSnapshot is an open pull request on acme/api by ana-dev, as GitHub last updated it at updatedAt.
func PullRequestSnapshot(number int, title string, updatedAt time.Time) githubsync.PullRequestSnapshot {
	return githubsync.PullRequestSnapshot{
		Repository: PullRequestRepository, Number: number, Title: title, State: githubsync.PullRequestOpen,
		Author:          githubsync.PullRequestAuthor{Login: PullRequestAuthorLogin, AvatarURL: PullRequestAuthorAvatarURL},
		GithubUpdatedAt: updatedAt,
	}
}

// StorePullRequest saves an open pull request snapshot and returns its id.
func StorePullRequest(t testing.TB, db platform.DBTX, number int) int64 {
	t.Helper()
	saved, err := githubsync.GithubPullRequestRepository{}.Upsert(context.Background(), db,
		PullRequestSnapshot(number, "Pull request", boardtest.FixedTime), boardtest.FixedTime)
	if err != nil {
		t.Fatalf("store pull request %d: %v", number, err)
	}
	return saved.ID
}
