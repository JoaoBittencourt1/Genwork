package boards

import (
	"context"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ColumnTaskCounter counts the active and archived tasks of a column; both keep it from being deleted.
type ColumnTaskCounter interface {
	CountInColumn(ctx context.Context, db platform.DBTX, columnID int64) (int, error)
}

type BoardService struct {
	db      *pgxpool.Pool
	boards  BoardRepository
	columns ColumnRepository
	tasks   ColumnTaskCounter
	now     func() time.Time
}

func NewBoardService(db *pgxpool.Pool, taskCounter ColumnTaskCounter, now func() time.Time) *BoardService {
	return &BoardService{db: db, tasks: taskCounter, now: now}
}

func (service *BoardService) CreateBoard(ctx context.Context, name, createdBy string) (Board, []Column, error) {
	at := service.now()
	var board Board
	var columns []Column
	err := pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		var err error
		board, err = service.boards.Create(ctx, tx, NewBoard{Name: name, CreatedBy: createdBy, At: at})
		if err != nil {
			return err
		}
		for position, preset := range DefaultColumnPresets {
			column, err := service.columns.Create(ctx, tx, NewColumn{BoardID: board.ID, Preset: &preset, Position: position, At: at})
			if err != nil {
				return err
			}
			columns = append(columns, column)
		}
		return nil
	})
	return board, columns, err
}

func (service *BoardService) ListBoards(ctx context.Context) ([]Board, error) {
	return service.boards.List(ctx, service.db)
}

func (service *BoardService) FindBoard(ctx context.Context, token string) (Board, error) {
	return service.boards.FindByToken(ctx, service.db, token)
}

// UpdateBoard leaves the board unchanged when newName is nil.
func (service *BoardService) UpdateBoard(ctx context.Context, token string, newName *string) (Board, error) {
	board, err := service.boards.FindByToken(ctx, service.db, token)
	if err != nil || newName == nil {
		return board, err
	}
	return service.boards.Rename(ctx, service.db, board.ID, *newName, service.now())
}

func (service *BoardService) DeleteBoard(ctx context.Context, token string) error {
	board, err := service.boards.FindByToken(ctx, service.db, token)
	if err != nil {
		return err
	}
	return service.boards.SoftDelete(ctx, service.db, board.ID, service.now())
}

func (service *BoardService) AddColumn(ctx context.Context, boardToken, name string) (Column, error) {
	var column Column
	err := pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		board, err := service.boards.FindByToken(ctx, tx, boardToken)
		if err != nil {
			return err
		}
		if err := platform.LockBoard(ctx, tx, board.ID); err != nil {
			return err
		}
		count, err := service.columns.CountInBoard(ctx, tx, board.ID)
		if err != nil {
			return err
		}
		column, err = service.columns.Create(ctx, tx, NewColumn{BoardID: board.ID, Name: &name, Position: count, At: service.now()})
		return err
	})
	return column, err
}

func (service *BoardService) UpdateColumn(ctx context.Context, token string, change ColumnNameChange) (Column, error) {
	column, err := service.columns.FindByToken(ctx, service.db, token)
	if err != nil || !change.Requested {
		return column, err
	}
	if change.Name == nil && column.Preset == nil {
		return Column{}, errCustomColumnNeedsName
	}
	return service.columns.Rename(ctx, service.db, column.ID, change.Name, service.now())
}

func (service *BoardService) MoveColumn(ctx context.Context, token string, targetPosition int) error {
	return service.withLockedColumn(ctx, token, func(tx pgx.Tx, column Column) error {
		count, err := service.columns.CountInBoard(ctx, tx, column.BoardID)
		if err != nil {
			return err
		}
		target := platform.ClampPosition(targetPosition, count-1)
		err = platform.ReorderWithin(column.Position, target, func(from, to, delta int) error {
			return service.columns.ShiftPositions(ctx, tx, column.BoardID, from, to, delta)
		})
		if err != nil {
			return err
		}
		return service.columns.SetPosition(ctx, tx, column.ID, target, service.now())
	})
}

func (service *BoardService) DeleteColumn(ctx context.Context, token string) error {
	return service.withLockedColumn(ctx, token, func(tx pgx.Tx, column Column) error {
		columnCount, err := service.columns.CountInBoard(ctx, tx, column.BoardID)
		if err != nil {
			return err
		}
		if columnCount <= 1 {
			return ErrLastColumn
		}
		taskCount, err := service.tasks.CountInColumn(ctx, tx, column.ID)
		if err != nil {
			return err
		}
		if taskCount > 0 {
			return ErrColumnNotEmpty
		}
		if err := service.columns.SoftDelete(ctx, tx, column.ID, service.now()); err != nil {
			return err
		}
		return service.columns.ShiftPositions(ctx, tx, column.BoardID, column.Position+1, platform.EndOfList, -1)
	})
}

// withLockedColumn re-reads the column after locking its board, so the position it acts on is current.
func (service *BoardService) withLockedColumn(ctx context.Context, token string, change func(tx pgx.Tx, column Column) error) error {
	return pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		column, err := service.columns.FindByToken(ctx, tx, token)
		if err != nil {
			return err
		}
		if err := platform.LockBoard(ctx, tx, column.BoardID); err != nil {
			return err
		}
		column, err = service.columns.FindByID(ctx, tx, column.ID)
		if err != nil {
			return err
		}
		return change(tx, column)
	})
}
