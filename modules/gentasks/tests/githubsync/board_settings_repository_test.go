package githubsync_test

import (
	"context"
	"slices"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/boards"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/boardtest"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsync"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform/dbtest"
)

func TestBoardGithubSettingsDefaultWhenNeverSaved(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	board, columns := boardtest.NewBoardWithDefaultColumns(t, db, "Platform")

	settings, err := githubsync.BoardGithubSettingsRepository{}.FindByBoard(context.Background(), db, board.ID)

	if err != nil || settings.Saved || settings.Opened != nil || settings.ReviewRequested != nil {
		t.Fatalf("settings = %+v, err = %v", settings, err)
	}
	if settings.Merged == nil || settings.Merged.Token != columns[2].Token || settings.Merged.Position != 2 {
		t.Errorf("merged = %+v, want the done column", settings.Merged)
	}
	if !slices.Equal(settings.FinalBranches, []string{"main", "master"}) {
		t.Errorf("final branches = %v", settings.FinalBranches)
	}
}

func TestBoardGithubSettingsDefaultWithoutDoneColumn(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	board, columns := boardtest.NewBoardWithDefaultColumns(t, db, "Platform")
	if err := (boards.ColumnRepository{}).SoftDelete(context.Background(), db, columns[2].ID, boardtest.FixedTime); err != nil {
		t.Fatal(err)
	}

	settings, err := githubsync.BoardGithubSettingsRepository{}.FindByBoard(context.Background(), db, board.ID)

	if err != nil || settings.Merged != nil {
		t.Errorf("merged = %+v, err = %v", settings.Merged, err)
	}
}

func TestBoardGithubSettingsSaveAndOverwrite(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	repository := githubsync.BoardGithubSettingsRepository{}
	board, columns := boardtest.NewBoardWithDefaultColumns(t, db, "Platform")

	err := repository.Save(ctx, db, githubsync.SavedBoardGithubSettings{
		BoardID: board.ID, OpenedColumnID: &columns[1].ID, ReviewRequestedColumnID: &columns[1].ID, MergedColumnID: nil,
		FinalBranches: []string{"main", "staging"}, At: boardtest.FixedTime,
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	err = repository.Save(ctx, db, githubsync.SavedBoardGithubSettings{
		BoardID: board.ID, OpenedColumnID: &columns[1].ID, ReviewRequestedColumnID: &columns[2].ID, MergedColumnID: nil,
		FinalBranches: []string{"staging"}, At: boardtest.FixedTime,
	})
	if err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	settings, err := repository.FindByBoard(ctx, db, board.ID)

	if err != nil || !settings.Saved || settings.Opened == nil || settings.Opened.ID != columns[1].ID || settings.ReviewRequested.Token != columns[2].Token {
		t.Fatalf("settings = %+v, err = %v", settings, err)
	}
	if settings.Merged != nil {
		t.Errorf("a saved null merged column must not fall back to the default, got %+v", settings.Merged)
	}
	if !slices.Equal(settings.FinalBranches, []string{"staging"}) {
		t.Errorf("final branches = %v", settings.FinalBranches)
	}
}

func TestBoardGithubSettingsTreatDeletedColumnsAsUnset(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	repository := githubsync.BoardGithubSettingsRepository{}
	board, columns := boardtest.NewBoardWithDefaultColumns(t, db, "Platform")
	if err := repository.Save(ctx, db, githubsync.SavedBoardGithubSettings{
		BoardID: board.ID, OpenedColumnID: &columns[1].ID, FinalBranches: []string{"main"}, At: boardtest.FixedTime,
	}); err != nil {
		t.Fatal(err)
	}
	if err := (boards.ColumnRepository{}).SoftDelete(ctx, db, columns[1].ID, boardtest.FixedTime); err != nil {
		t.Fatal(err)
	}

	settings, err := repository.FindByBoard(ctx, db, board.ID)

	if err != nil || settings.Opened != nil {
		t.Errorf("opened = %+v, err = %v", settings.Opened, err)
	}
}

func TestBoardGithubSettingsListByBoardsInOneCall(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	repository := githubsync.BoardGithubSettingsRepository{}
	platformBoard, platformColumns := boardtest.NewBoardWithDefaultColumns(t, db, "Platform")
	ops, _ := boardtest.NewBoardWithDefaultColumns(t, db, "Ops")
	if err := repository.Save(ctx, db, githubsync.SavedBoardGithubSettings{
		BoardID: platformBoard.ID, OpenedColumnID: &platformColumns[1].ID, FinalBranches: []string{"main"}, At: boardtest.FixedTime,
	}); err != nil {
		t.Fatal(err)
	}

	byBoard, err := repository.ListByBoards(ctx, db, []int64{platformBoard.ID, ops.ID})

	if err != nil || len(byBoard) != 2 || !byBoard[platformBoard.ID].Saved || byBoard[ops.ID].Saved || byBoard[platformBoard.ID].Opened == nil {
		t.Fatalf("byBoard = %+v, err = %v", byBoard, err)
	}
}
