package labels_test

import (
	"context"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/boardtest"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/labels"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform/dbtest"
)

func TestLabelNamesAreUniquePerBoardIgnoringCase(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	repository := labels.LabelRepository{}
	board, _ := boardtest.NewBoardWithDefaultColumns(t, db, "Board")
	other, _ := boardtest.NewBoardWithDefaultColumns(t, db, "Other")
	bug := boardtest.NewLabel(t, db, board, "Bug", labels.ColorRed)

	if _, err := repository.Create(ctx, db, labels.NewLabel{BoardID: board.ID, Name: "bug", Color: labels.ColorBlue}); err != labels.ErrLabelNameTaken {
		t.Fatalf("duplicate error = %v", err)
	}
	boardtest.NewLabel(t, db, other, "bug", labels.ColorBlue)

	if err := repository.SoftDelete(ctx, db, bug.ID, boardtest.FixedTime); err != nil {
		t.Fatalf("delete: %v", err)
	}
	boardtest.NewLabel(t, db, board, "bug", labels.ColorBlue)
	if _, err := repository.FindByToken(ctx, db, bug.Token); err != labels.ErrLabelNotFound {
		t.Errorf("deleted label lookup error = %v", err)
	}
}

func TestLabelRepositoryUpdatesAndFindsByTokens(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	repository := labels.LabelRepository{}
	board, _ := boardtest.NewBoardWithDefaultColumns(t, db, "Board")
	other, _ := boardtest.NewBoardWithDefaultColumns(t, db, "Other")
	bug := boardtest.NewLabel(t, db, board, "Bug", labels.ColorRed)
	feature := boardtest.NewLabel(t, db, board, "Feature", labels.ColorGreen)
	foreign := boardtest.NewLabel(t, db, other, "Foreign", labels.ColorGray)

	bug.Color = labels.ColorOrange
	updated, err := repository.Update(ctx, db, bug, boardtest.FixedTime)
	if err != nil || updated.Color != labels.ColorOrange || updated.Name != "Bug" {
		t.Fatalf("updated = %+v, err = %v", updated, err)
	}
	feature.Name = "bug"
	if _, err := repository.Update(ctx, db, feature, boardtest.FixedTime); err != labels.ErrLabelNameTaken {
		t.Errorf("rename to a taken name error = %v", err)
	}

	found, err := repository.FindInBoardByTokens(ctx, db, board.ID, []string{bug.Token, foreign.Token, "unknown"})
	if err != nil || len(found) != 1 || found[0].Token != bug.Token {
		t.Errorf("found = %+v, err = %v", found, err)
	}
	listed, _ := repository.ListByBoard(ctx, db, board.ID)
	if len(listed) != 2 || listed[0].Name != "Bug" || listed[1].Name != "Feature" {
		t.Errorf("listed = %+v", listed)
	}
}

func TestTaskLabelsReplaceDetachAndBatchLoad(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	repository := labels.TaskLabelRepository{}
	board, columns := boardtest.NewBoardWithDefaultColumns(t, db, "Board")
	created := boardtest.NewTasks(t, db, columns[0], "First", "Second")
	bug := boardtest.NewLabel(t, db, board, "Bug", labels.ColorRed)
	feature := boardtest.NewLabel(t, db, board, "Feature", labels.ColorGreen)

	if err := repository.ReplaceForTask(ctx, db, created[0].ID, []int64{bug.ID, feature.ID}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if err := repository.ReplaceForTask(ctx, db, created[1].ID, []int64{bug.ID}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if err := repository.ReplaceForTask(ctx, db, created[0].ID, []int64{feature.ID}); err != nil {
		t.Fatalf("replace again: %v", err)
	}

	byTask, err := repository.ListForTasks(ctx, db, []int64{created[0].ID, created[1].ID})
	if err != nil || len(byTask[created[0].ID]) != 1 || byTask[created[0].ID][0].Token != feature.Token || len(byTask[created[1].ID]) != 1 {
		t.Fatalf("by task = %+v, err = %v", byTask, err)
	}

	if err := repository.DetachLabel(ctx, db, bug.ID); err != nil {
		t.Fatalf("detach: %v", err)
	}
	byTask, _ = repository.ListForTasks(ctx, db, []int64{created[1].ID})
	if len(byTask[created[1].ID]) != 0 {
		t.Errorf("detached label still listed: %+v", byTask)
	}
}
