package boards_test

import (
	"context"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/boards"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/boardtest"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform/dbtest"
)

func TestBoardRepositoryHidesDeletedBoards(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	repository := boards.BoardRepository{}
	zeta, _ := boardtest.NewBoardWithDefaultColumns(t, db, "Zeta")
	alpha, _ := boardtest.NewBoardWithDefaultColumns(t, db, "alpha")
	gone, goneColumns := boardtest.NewBoardWithDefaultColumns(t, db, "Gone")

	if err := repository.SoftDelete(ctx, db, gone.ID, boardtest.FixedTime); err != nil {
		t.Fatalf("delete: %v", err)
	}

	listed, err := repository.List(ctx, db)
	if err != nil || len(listed) != 2 || listed[0].Token != alpha.Token || listed[1].Token != zeta.Token {
		t.Fatalf("listed = %+v, err = %v", listed, err)
	}
	if _, err := repository.FindByToken(ctx, db, gone.Token); err != boards.ErrBoardNotFound {
		t.Errorf("deleted board lookup error = %v", err)
	}
	if _, err := (boards.ColumnRepository{}).FindByToken(ctx, db, goneColumns[0].Token); err != boards.ErrColumnNotFound {
		t.Errorf("column of a deleted board lookup error = %v", err)
	}
}

func TestBoardRepositoryUpdatesNameAndTaskPrefix(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	board, _ := boardtest.NewBoardWithDefaultColumns(t, db, "Old")
	taken, _ := boardtest.NewBoardWithDefaultColumns(t, db, "Taken")
	later := boardtest.FixedTime.Add(time.Hour)
	board.Name, board.TaskPrefix = "New", "NEW"

	updated, err := boards.BoardRepository{}.Update(context.Background(), db, board, later)

	if err != nil || updated.Name != "New" || updated.TaskPrefix != "NEW" || !updated.UpdatedAt.Equal(later) || updated.CreatedBy != boardtest.CreatorToken {
		t.Errorf("updated = %+v, err = %v", updated, err)
	}
	found, _ := boards.BoardRepository{}.FindByID(context.Background(), db, board.ID)
	if found.Name != "New" || found.TaskPrefix != "NEW" {
		t.Errorf("found = %+v", found)
	}
	board.TaskPrefix = taken.TaskPrefix
	if _, err := (boards.BoardRepository{}).Update(context.Background(), db, board, later); err != boards.ErrTaskPrefixTaken {
		t.Errorf("taken prefix error = %v", err)
	}
}

func TestBoardRepositoryHandsOutTaskNumbersInSequence(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	repository := boards.BoardRepository{}
	board, _ := boardtest.NewBoardWithDefaultColumns(t, db, "Board")
	other, _ := boardtest.NewBoardWithDefaultColumns(t, db, "Other")

	first, _ := repository.TakeNextTaskNumber(ctx, db, board.ID)
	second, _ := repository.TakeNextTaskNumber(ctx, db, board.ID)
	otherFirst, err := repository.TakeNextTaskNumber(ctx, db, other.ID)

	if err != nil || first != 1 || second != 2 || otherFirst != 1 {
		t.Errorf("numbers = %d, %d, %d, err = %v", first, second, otherFirst, err)
	}
	prefixes, err := repository.ListTaskPrefixesStartingWith(ctx, db, "BOA")
	if err != nil || len(prefixes) != 1 || prefixes[0] != board.TaskPrefix {
		t.Errorf("prefixes = %v, err = %v", prefixes, err)
	}
}

func TestColumnRepositoryKeepsOrderAndShiftsPositions(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	repository := boards.ColumnRepository{}
	board, columns := boardtest.NewBoardWithDefaultColumns(t, db, "Board")

	if err := repository.ShiftPositions(ctx, db, board.ID, 1, 2, 1); err != nil {
		t.Fatalf("shift: %v", err)
	}
	name := "Review"
	review, err := repository.Create(ctx, db, boards.NewColumn{BoardID: board.ID, Name: &name, Position: 1, At: boardtest.FixedTime})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	listed, _ := repository.ListByBoard(ctx, db, board.ID)
	if len(listed) != 4 || listed[0].Token != columns[0].Token || listed[1].Token != review.Token || listed[3].Token != columns[2].Token {
		t.Fatalf("order = %+v", listed)
	}
	first, _ := repository.FindFirstInBoard(ctx, db, board.ID)
	count, _ := repository.CountInBoard(ctx, db, board.ID)
	if first.Token != columns[0].Token || *first.Preset != boards.PresetBacklog || first.Name != nil || count != 4 {
		t.Errorf("first = %+v, count = %d", first, count)
	}
}

func TestColumnRepositoryRenamesAndDeletes(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	repository := boards.ColumnRepository{}
	board, columns := boardtest.NewBoardWithDefaultColumns(t, db, "Board")
	name := "Next up"

	renamed, err := repository.Rename(ctx, db, columns[1].ID, &name, boardtest.FixedTime)
	if err != nil || *renamed.Name != "Next up" || *renamed.Preset != boards.PresetTodo {
		t.Fatalf("renamed = %+v, err = %v", renamed, err)
	}
	if err := repository.SetPosition(ctx, db, columns[2].ID, 1, boardtest.FixedTime); err != nil {
		t.Fatalf("set position: %v", err)
	}
	if err := repository.SoftDelete(ctx, db, columns[1].ID, boardtest.FixedTime); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := repository.FindByID(ctx, db, columns[1].ID); err != boards.ErrColumnNotFound {
		t.Errorf("deleted column lookup error = %v", err)
	}
	listed, _ := repository.ListByBoard(ctx, db, board.ID)
	if len(listed) != 2 || listed[1].Position != 1 {
		t.Errorf("listed = %+v", listed)
	}
}
