package tasks_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/boards"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/boardtest"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform/dbtest"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/tasks"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"slices"
)

func TestTaskRepositoryOrdersActiveTasksByColumnThenPosition(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	board, columns := boardtest.NewBoardWithDefaultColumns(t, db, "Board")
	boardtest.NewTasks(t, db, columns[1], "Todo A", "Todo B")
	boardtest.NewTasks(t, db, columns[0], "Backlog A")

	active, err := tasks.TaskRepository{}.ListActiveByBoard(context.Background(), db, board.ID)

	if err != nil || len(active) != 3 || active[0].Title != "Backlog A" || active[2].Title != "Todo B" {
		t.Fatalf("active = %+v, err = %v", active, err)
	}
	if active[0].ColumnToken != columns[0].Token || active[0].Urgency != tasks.UrgencyNone || active[0].CreatedBy != boardtest.CreatorToken {
		t.Errorf("first = %+v", active[0])
	}
}

func TestTaskRepositoryShiftsAndPlacesTasks(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	repository := tasks.TaskRepository{}
	board, columns := boardtest.NewBoardWithDefaultColumns(t, db, "Board")
	created := boardtest.NewTasks(t, db, columns[0], "A", "B", "C")

	if err := repository.ShiftPositions(ctx, db, columns[0].ID, 3, 2, -1); err != nil {
		t.Fatalf("empty shift: %v", err)
	}
	if err := repository.ShiftPositions(ctx, db, columns[0].ID, 1, 2, -1); err != nil {
		t.Fatalf("shift: %v", err)
	}
	if err := repository.Place(ctx, db, created[0].ID, columns[0].ID, 2, boardtest.FixedTime); err != nil {
		t.Fatalf("place: %v", err)
	}

	if titles := boardtest.ActiveTitlesByColumn(t, db, board.ID, columns[0].ID); !slices.Equal(titles, []string{"B", "C", "A"}) {
		t.Errorf("titles = %v", titles)
	}
}

func TestTaskRepositoryArchivesAndDeletes(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	repository := tasks.TaskRepository{}
	board, columns := boardtest.NewBoardWithDefaultColumns(t, db, "Board")
	created := boardtest.NewTasks(t, db, columns[0], "Old", "Older", "Deleted")

	if err := repository.Archive(ctx, db, created[0].ID, boardtest.FixedTime); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if err := repository.Archive(ctx, db, created[1].ID, boardtest.FixedTime.Add(time.Hour)); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if err := repository.SoftDelete(ctx, db, created[2].ID, boardtest.FixedTime); err != nil {
		t.Fatalf("delete: %v", err)
	}

	archived, _ := repository.ListArchivedByBoard(ctx, db, board.ID)
	if len(archived) != 2 || archived[0].Title != "Older" || !archived[0].IsArchived() || archived[0].Position != nil {
		t.Errorf("archived = %+v", archived)
	}
	active, _ := repository.CountActiveInColumn(ctx, db, columns[0].ID)
	total, _ := repository.CountInColumn(ctx, db, columns[0].ID)
	if active != 0 || total != 2 {
		t.Errorf("active = %d, total = %d", active, total)
	}
	if _, err := repository.FindByToken(ctx, db, created[2].Token); err != tasks.ErrTaskNotFound {
		t.Errorf("deleted task lookup error = %v", err)
	}

	description := "details"
	task := archived[1]
	task.Title, task.Description, task.Urgency = "Renamed", &description, tasks.UrgencyHigh
	if err := repository.UpdateDetails(ctx, db, task, boardtest.FixedTime); err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := repository.Place(ctx, db, task.ID, columns[1].ID, 0, boardtest.FixedTime); err != nil {
		t.Fatalf("restore: %v", err)
	}
	restored, _ := repository.FindByToken(ctx, db, task.Token)
	if restored.Title != "Renamed" || *restored.Description != "details" || restored.Urgency != tasks.UrgencyHigh || restored.IsArchived() || restored.ColumnToken != columns[1].Token {
		t.Errorf("restored = %+v", restored)
	}
}

func appendTaskUnderBoardLock(ctx context.Context, db *pgxpool.Pool, boardID, columnID int64, title string) error {
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if err := platform.LockBoard(ctx, tx, boardID); err != nil {
			return err
		}
		count, err := tasks.TaskRepository{}.CountActiveInColumn(ctx, tx, columnID)
		if err != nil {
			return err
		}
		number, err := boards.BoardRepository{}.TakeNextTaskNumber(ctx, tx, boardID)
		if err != nil {
			return err
		}
		time.Sleep(5 * time.Millisecond)
		_, err = tasks.TaskRepository{}.Create(ctx, tx, tasks.NewTask{
			BoardID: boardID, Number: number, ColumnID: columnID, Title: title, Urgency: tasks.UrgencyNone, Position: count,
			CreatedBy: boardtest.CreatorToken, At: boardtest.FixedTime,
		})
		return err
	})
}

func TestBoardLockSerializesConcurrentPositionChanges(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	board, columns := boardtest.NewBoardWithDefaultColumns(t, db, "Board")
	var wait sync.WaitGroup
	errs := make(chan error, 8)

	for range 8 {
		wait.Go(func() {
			errs <- appendTaskUnderBoardLock(context.Background(), db, board.ID, columns[0].ID, "Concurrent")
		})
	}
	wait.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	if titles := boardtest.ActiveTitlesByColumn(t, db, board.ID, columns[0].ID); len(titles) != 8 {
		t.Errorf("titles = %v", titles)
	}
}
