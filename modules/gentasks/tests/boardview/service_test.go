package boardview_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/boards"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/boardtest"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/boardview"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsync"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsynctest"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/labels"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform/dbtest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type countingDB struct {
	platform.DBTX
	queries int
}

func (db *countingDB) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	db.queries++
	return db.DBTX.Exec(ctx, sql, arguments...)
}

func (db *countingDB) Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error) {
	db.queries++
	return db.DBTX.Query(ctx, sql, arguments...)
}

func (db *countingDB) QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row {
	db.queries++
	return db.DBTX.QueryRow(ctx, sql, arguments...)
}

// addLinkedTasks adds tasks that each have a label and a linked pull request.
func addLinkedTasks(t *testing.T, db platform.DBTX, board boards.Board, columns []boards.Column, count int) {
	t.Helper()
	ctx := context.Background()
	label := boardtest.NewLabel(t, db, board, fmt.Sprintf("Label %d", count), labels.ColorBlue)
	pullRequest := githubsynctest.StorePullRequest(t, db, count)
	for index := range count {
		created := boardtest.NewTasks(t, db, columns[index%len(columns)], fmt.Sprintf("Task %d", index))
		if err := (labels.TaskLabelRepository{}).ReplaceForTask(ctx, db, created[0].ID, []int64{label.ID}); err != nil {
			t.Fatalf("label task: %v", err)
		}
		if err := (githubsync.TaskPullRequestRepository{}).Link(ctx, db, pullRequest, []int64{created[0].ID}, boardtest.FixedTime); err != nil {
			t.Fatalf("link task: %v", err)
		}
	}
}

func queriesToRead(t *testing.T, db platform.DBTX, token string) (int, boardview.BoardView) {
	t.Helper()
	counting := &countingDB{DBTX: db}
	view, err := boardview.NewBoardViewService(counting).ReadBoard(context.Background(), token)
	if err != nil {
		t.Fatalf("read board: %v", err)
	}
	return counting.queries, view
}

func TestReadingABoardUsesAFixedNumberOfQueries(t *testing.T) {
	pool := dbtest.NewMigratedSchema(t)
	small, smallColumns := boardtest.NewBoardWithDefaultColumns(t, pool, "Small")
	large, largeColumns := boardtest.NewBoardWithDefaultColumns(t, pool, "Large")
	addLinkedTasks(t, pool, small, smallColumns, 2)
	addLinkedTasks(t, pool, large, largeColumns, 40)

	smallQueries, smallView := queriesToRead(t, pool, small.Token)
	largeQueries, largeView := queriesToRead(t, pool, large.Token)

	if smallQueries != largeQueries {
		t.Errorf("queries grow with the board: %d for 2 tasks, %d for 40", smallQueries, largeQueries)
	}
	if len(smallView.Tasks) != 2 || len(largeView.Tasks) != 40 || len(largeView.Tasks[39].Labels) != 1 || len(largeView.Labels) != 1 {
		t.Errorf("small = %d tasks, large = %d tasks", len(smallView.Tasks), len(largeView.Tasks))
	}
	if linked := largeView.PullRequests[largeView.Tasks[39].ID]; len(linked) != 1 || linked[0].PullRequest.Number != 40 {
		t.Errorf("pull requests of the last task = %+v", linked)
	}
}
