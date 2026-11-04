package githubsync_test

import (
	"context"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/boardtest"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsync"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsynctest"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform/dbtest"
)

func TestGithubPullRequestRepositoryStoresSnapshots(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	repository := githubsync.GithubPullRequestRepository{}

	saved, err := repository.Upsert(ctx, db, githubsynctest.PullRequestSnapshot(12, "GEN-1 sign in", boardtest.FixedTime), boardtest.FixedTime)

	if err != nil || !saved.Current || saved.ID == 0 {
		t.Fatalf("saved = %+v, err = %v", saved, err)
	}
	stored, found, err := repository.FindByNumber(ctx, db, githubsynctest.PullRequestRepository, 12)
	if err != nil || !found || stored.ID != saved.ID || len(stored.Token) != 22 || *stored.Title != "GEN-1 sign in" ||
		*stored.State != githubsync.PullRequestOpen || stored.Author.Login != githubsynctest.PullRequestAuthorLogin ||
		stored.Author.AvatarURL != githubsynctest.PullRequestAuthorAvatarURL || !stored.GithubUpdatedAt.Equal(boardtest.FixedTime) {
		t.Errorf("stored = %+v, found = %v, err = %v", stored, found, err)
	}
}

func TestGithubPullRequestRepositoryIgnoresOlderSnapshots(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	repository := githubsync.GithubPullRequestRepository{}
	later := boardtest.FixedTime.Add(5 * time.Minute)
	first, _ := repository.Upsert(ctx, db, githubsynctest.PullRequestSnapshot(12, "Newer title", later), later)

	older, err := repository.Upsert(ctx, db, githubsynctest.PullRequestSnapshot(12, "Older title", boardtest.FixedTime), later)
	if err != nil || older.Current || older.ID != first.ID {
		t.Fatalf("older = %+v, err = %v", older, err)
	}
	same, err := repository.Upsert(ctx, db, githubsynctest.PullRequestSnapshot(12, "Same time", later), later)
	if err != nil || !same.Current || same.ID != first.ID {
		t.Fatalf("same = %+v, err = %v", same, err)
	}

	stored, _, _ := repository.FindByNumber(ctx, db, githubsynctest.PullRequestRepository, 12)
	if *stored.Title != "Same time" {
		t.Errorf("title = %q", *stored.Title)
	}
}

func TestGithubPullRequestRepositoryUpdatesOnlyKnownPullRequests(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	repository := githubsync.GithubPullRequestRepository{}
	later := boardtest.FixedTime.Add(time.Minute)

	_, found, err := repository.UpdateKnown(ctx, db, githubsynctest.PullRequestSnapshot(12, "Unknown", later), later)
	if err != nil || found {
		t.Fatalf("found = %v, err = %v", found, err)
	}
	if _, stored, _ := repository.FindByNumber(ctx, db, githubsynctest.PullRequestRepository, 12); stored {
		t.Error("an unknown pull request was stored")
	}

	id := githubsynctest.StorePullRequest(t, db, 12)
	closed := githubsynctest.PullRequestSnapshot(12, "Closed", later)
	closed.State = githubsync.PullRequestClosed
	saved, found, err := repository.UpdateKnown(ctx, db, closed, later)
	if err != nil || !found || !saved.Current || saved.ID != id {
		t.Fatalf("saved = %+v, found = %v, err = %v", saved, found, err)
	}
	stale, found, err := repository.UpdateKnown(ctx, db, githubsynctest.PullRequestSnapshot(12, "Stale", boardtest.FixedTime), later)
	if err != nil || !found || stale.Current || stale.ID != id {
		t.Fatalf("stale = %+v, found = %v, err = %v", stale, found, err)
	}
	stored, _, _ := repository.FindByNumber(ctx, db, githubsynctest.PullRequestRepository, 12)
	if *stored.Title != "Closed" || *stored.State != githubsync.PullRequestClosed {
		t.Errorf("stored = %+v", stored)
	}
}
