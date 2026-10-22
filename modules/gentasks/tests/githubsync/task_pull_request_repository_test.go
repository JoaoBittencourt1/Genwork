package githubsync_test

import (
	"context"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/boardtest"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsync"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform/dbtest"
)

func TestTaskPullRequestRepositoryRecordsOneLinkPerTaskAndPullRequest(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	repository := githubsync.TaskPullRequestRepository{}
	_, columns := boardtest.NewBoardWithDefaultColumns(t, db, "Platform")
	task := boardtest.NewTasks(t, db, columns[1], "Login")[0]
	later := boardtest.FixedTime.Add(time.Hour)

	moves := []githubsync.TaskPullRequestMove{
		{TaskID: task.ID, Repository: "acme/api", PullRequestNumber: 12, Trigger: githubsync.TriggerOpened, At: boardtest.FixedTime},
		{TaskID: task.ID, Repository: "acme/api", PullRequestNumber: 12, Trigger: githubsync.TriggerReviewRequested, At: later},
		{TaskID: task.ID, Repository: "acme/web", PullRequestNumber: 3, Trigger: githubsync.TriggerOpened, At: boardtest.FixedTime},
	}
	for _, move := range moves {
		if err := repository.RecordMove(ctx, db, move); err != nil {
			t.Fatalf("record %+v: %v", move, err)
		}
	}
	links, err := repository.ListByTask(ctx, db, task.ID)

	if err != nil || len(links) != 2 {
		t.Fatalf("links = %+v, err = %v", links, err)
	}
	api := links[0]
	if api.Repository != "acme/api" || api.PullRequestNumber != 12 || api.LastMovedEvent != githubsync.TriggerReviewRequested ||
		!api.MovedAt.Equal(later) || api.MergeMovedAt != nil || len(api.Token) != 22 {
		t.Errorf("api link = %+v", api)
	}
}

func TestTaskPullRequestRepositoryLocksTasksMovedByAMerge(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	repository := githubsync.TaskPullRequestRepository{}
	_, columns := boardtest.NewBoardWithDefaultColumns(t, db, "Platform")
	created := boardtest.NewTasks(t, db, columns[1], "Merged", "Open", "Untouched")
	merged, open := created[0], created[1]

	for _, move := range []githubsync.TaskPullRequestMove{
		{TaskID: merged.ID, Repository: "acme/api", PullRequestNumber: 1, Trigger: githubsync.TriggerMerged, At: boardtest.FixedTime},
		{TaskID: merged.ID, Repository: "acme/api", PullRequestNumber: 2, Trigger: githubsync.TriggerOpened, At: boardtest.FixedTime},
		{TaskID: open.ID, Repository: "acme/api", PullRequestNumber: 3, Trigger: githubsync.TriggerReviewRequested, At: boardtest.FixedTime},
	} {
		if err := repository.RecordMove(ctx, db, move); err != nil {
			t.Fatal(err)
		}
	}

	locked, err := repository.ListMergeLockedTaskIDs(ctx, db, []int64{merged.ID, open.ID, created[2].ID})

	if err != nil || len(locked) != 1 || !locked[merged.ID] {
		t.Errorf("locked = %v, err = %v", locked, err)
	}
	links, _ := repository.ListByTask(ctx, db, merged.ID)
	if links[0].MergeMovedAt == nil || !links[0].MergeMovedAt.Equal(boardtest.FixedTime) {
		t.Errorf("merge link = %+v", links[0])
	}
}
