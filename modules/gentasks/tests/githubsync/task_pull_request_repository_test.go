package githubsync_test

import (
	"context"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/boardtest"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsync"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsynctest"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform/dbtest"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/tasks"
)

func activeLinks(t *testing.T, db platform.DBTX, taskIDs ...int64) map[int64][]githubsync.LinkedPullRequest {
	t.Helper()
	linked, err := githubsync.TaskPullRequestRepository{}.ListActiveForTasks(context.Background(), db, taskIDs)
	if err != nil {
		t.Fatalf("list active links: %v", err)
	}
	return linked
}

func TestTaskPullRequestRepositoryRecordsOneLinkPerTaskAndPullRequest(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	repository := githubsync.TaskPullRequestRepository{}
	_, columns := boardtest.NewBoardWithDefaultColumns(t, db, "Platform")
	task := boardtest.NewTasks(t, db, columns[1], "Login")[0]
	api, web := githubsynctest.StorePullRequest(t, db, 12), githubsynctest.StorePullRequest(t, db, 3)
	later := boardtest.FixedTime.Add(time.Hour)

	moves := []githubsync.TaskPullRequestMove{
		{TaskID: task.ID, PullRequestID: api, Trigger: githubsync.TriggerOpened, At: boardtest.FixedTime},
		{TaskID: task.ID, PullRequestID: api, Trigger: githubsync.TriggerReviewRequested, At: later},
		{TaskID: task.ID, PullRequestID: web, Trigger: githubsync.TriggerOpened, At: boardtest.FixedTime},
	}
	for _, move := range moves {
		if err := repository.RecordMove(ctx, db, move); err != nil {
			t.Fatalf("record %+v: %v", move, err)
		}
	}

	links := activeLinks(t, db, task.ID)[task.ID]
	if len(links) != 2 {
		t.Fatalf("links = %+v", links)
	}
	first := links[0]
	if first.PullRequest.Number != 3 || first.Link.PullRequestID != web || len(first.Link.Token) != 22 {
		t.Errorf("first link = %+v", first)
	}
	second := links[1].Link
	if second.PullRequestID != api || *second.LastMovedEvent != githubsync.TriggerReviewRequested || !second.MovedAt.Equal(later) ||
		second.MergeMovedAt != nil {
		t.Errorf("api link = %+v", second)
	}
}

func TestTaskPullRequestRepositoryLocksTasksMovedByAMerge(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	repository := githubsync.TaskPullRequestRepository{}
	board, columns := boardtest.NewBoardWithDefaultColumns(t, db, "Platform")
	created := boardtest.NewTasks(t, db, columns[1], "Merged", "Open", "Untouched")
	merged, open := created[0], created[1]
	first, second, third := githubsynctest.StorePullRequest(t, db, 1), githubsynctest.StorePullRequest(t, db, 2), githubsynctest.StorePullRequest(t, db, 3)

	for _, move := range []githubsync.TaskPullRequestMove{
		{TaskID: merged.ID, PullRequestID: first, Trigger: githubsync.TriggerMerged, At: boardtest.FixedTime},
		{TaskID: merged.ID, PullRequestID: second, Trigger: githubsync.TriggerOpened, At: boardtest.FixedTime},
		{TaskID: open.ID, PullRequestID: third, Trigger: githubsync.TriggerReviewRequested, At: boardtest.FixedTime},
	} {
		if err := repository.RecordMove(ctx, db, move); err != nil {
			t.Fatal(err)
		}
	}
	links := activeLinks(t, db, merged.ID)[merged.ID]
	if links[0].Link.MergeMovedAt == nil || !links[0].Link.MergeMovedAt.Equal(boardtest.FixedTime) {
		t.Errorf("merge link = %+v", links[0])
	}
	if err := repository.UnlinkUncited(ctx, db, first, nil, boardtest.FixedTime); err != nil {
		t.Fatal(err)
	}

	locked, err := repository.ListMergeLockedTaskIDs(ctx, db, []int64{merged.ID, open.ID, created[2].ID})

	if err != nil || len(locked) != 1 || !locked[merged.ID] {
		t.Errorf("locked = %v, err = %v", locked, err)
	}
	if remaining := activeLinks(t, db, merged.ID)[merged.ID]; len(remaining) != 1 || remaining[0].Link.PullRequestID != second {
		t.Errorf("remaining links of %s = %+v", board.TaskPrefix, remaining)
	}
}

func TestTaskPullRequestRepositoryLinksTasksWithoutMoves(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	repository := githubsync.TaskPullRequestRepository{}
	_, columns := boardtest.NewBoardWithDefaultColumns(t, db, "Platform")
	created := boardtest.NewTasks(t, db, columns[1], "Login", "Profile")
	pullRequest := githubsynctest.StorePullRequest(t, db, 12)

	for range 2 {
		if err := repository.Link(ctx, db, pullRequest, []int64{created[0].ID, created[1].ID}, boardtest.FixedTime); err != nil {
			t.Fatal(err)
		}
	}

	linked := activeLinks(t, db, created[0].ID, created[1].ID)
	for _, task := range created {
		links := linked[task.ID]
		if len(links) != 1 || links[0].Link.LastMovedEvent != nil || links[0].Link.MovedAt != nil || links[0].PullRequest.Number != 12 {
			t.Errorf("links of %s = %+v", task.Title, links)
		}
	}
}

func TestTaskPullRequestRepositoryUnlinksTasksNoLongerCited(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	repository := githubsync.TaskPullRequestRepository{}
	board, columns := boardtest.NewBoardWithDefaultColumns(t, db, "Platform")
	created := boardtest.NewTasks(t, db, columns[1], "Dropped", "Kept", "Archived")
	dropped, kept, archived := created[0], created[1], created[2]
	pullRequest := githubsynctest.StorePullRequest(t, db, 12)
	if err := repository.Link(ctx, db, pullRequest, []int64{dropped.ID, kept.ID, archived.ID}, boardtest.FixedTime); err != nil {
		t.Fatal(err)
	}
	if err := (tasks.TaskRepository{}).Archive(ctx, db, archived.ID, boardtest.FixedTime); err != nil {
		t.Fatal(err)
	}
	originalToken := activeLinks(t, db, dropped.ID)[dropped.ID][0].Link.Token

	cited := []githubsync.TaskKey{{Prefix: board.TaskPrefix, Number: kept.Number}, {Prefix: board.TaskPrefix, Number: archived.Number}}
	if err := repository.UnlinkUncited(ctx, db, pullRequest, cited, boardtest.FixedTime); err != nil {
		t.Fatal(err)
	}

	linked := activeLinks(t, db, dropped.ID, kept.ID, archived.ID)
	if len(linked[dropped.ID]) != 0 || len(linked[kept.ID]) != 1 || len(linked[archived.ID]) != 1 {
		t.Fatalf("linked = %+v", linked)
	}

	if err := repository.Link(ctx, db, pullRequest, []int64{dropped.ID}, boardtest.FixedTime); err != nil {
		t.Fatal(err)
	}
	relinked := activeLinks(t, db, dropped.ID)[dropped.ID]
	if len(relinked) != 1 || relinked[0].Link.Token == originalToken {
		t.Errorf("relinked = %+v", relinked)
	}
}
