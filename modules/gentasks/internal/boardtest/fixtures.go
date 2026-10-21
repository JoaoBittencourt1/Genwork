// Package boardtest builds boards, labels, and tasks straight through the repositories for tests (rule 26).
package boardtest

import (
	"context"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/boards"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/labels"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/tasks"
)

const CreatorToken = "creator-user-token"

var FixedTime = time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)

// NewBoardWithDefaultColumns creates a board with the backlog, todo, and done columns in order.
func NewBoardWithDefaultColumns(t *testing.T, db platform.DBTX, name string) (boards.Board, []boards.Column) {
	t.Helper()
	ctx := context.Background()
	board, err := boards.BoardRepository{}.Create(ctx, db, boards.NewBoard{
		Name: name, TaskPrefix: boards.DeriveTaskPrefix(name), CreatedBy: CreatorToken, At: FixedTime,
	})
	if err != nil {
		t.Fatalf("create board: %v", err)
	}
	columns := make([]boards.Column, 0, len(boards.DefaultColumnPresets))
	for position, preset := range boards.DefaultColumnPresets {
		column, err := boards.ColumnRepository{}.Create(ctx, db, boards.NewColumn{
			BoardID: board.ID, Preset: &preset, Position: position, At: FixedTime,
		})
		if err != nil {
			t.Fatalf("create column: %v", err)
		}
		columns = append(columns, column)
	}
	return board, columns
}

// NewTasks appends active tasks with the given titles at the end of the column.
func NewTasks(t *testing.T, db platform.DBTX, column boards.Column, titles ...string) []tasks.Task {
	t.Helper()
	ctx := context.Background()
	start, err := tasks.TaskRepository{}.CountActiveInColumn(ctx, db, column.ID)
	if err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	created := make([]tasks.Task, 0, len(titles))
	for offset, title := range titles {
		number, err := boards.BoardRepository{}.TakeNextTaskNumber(ctx, db, column.BoardID)
		if err != nil {
			t.Fatalf("take task number: %v", err)
		}
		task, err := tasks.TaskRepository{}.Create(ctx, db, tasks.NewTask{
			BoardID: column.BoardID, Number: number, ColumnID: column.ID, Title: title, Urgency: tasks.UrgencyNone,
			Position: start + offset, CreatedBy: CreatorToken, At: FixedTime,
		})
		if err != nil {
			t.Fatalf("create task: %v", err)
		}
		created = append(created, task)
	}
	return created
}

func NewLabel(t *testing.T, db platform.DBTX, board boards.Board, name string, color labels.LabelColor) labels.Label {
	t.Helper()
	label, err := labels.LabelRepository{}.Create(context.Background(), db, labels.NewLabel{
		BoardID: board.ID, Name: name, Color: color, At: FixedTime,
	})
	if err != nil {
		t.Fatalf("create label: %v", err)
	}
	return label
}

// ActiveTitlesByColumn lists the active task titles of a column in position order.
func ActiveTitlesByColumn(t *testing.T, db platform.DBTX, boardID, columnID int64) []string {
	t.Helper()
	active, err := tasks.TaskRepository{}.ListActiveByBoard(context.Background(), db, boardID)
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	var titles []string
	expectedPosition := 0
	for _, task := range active {
		if task.ColumnID != columnID {
			continue
		}
		if task.Position == nil || *task.Position != expectedPosition {
			t.Fatalf("task %q has position %v, want %d (positions must stay contiguous)", task.Title, task.Position, expectedPosition)
		}
		expectedPosition++
		titles = append(titles, task.Title)
	}
	return titles
}
