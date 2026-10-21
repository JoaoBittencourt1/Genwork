package boards

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
	"github.com/jackc/pgx/v5"
)

type BoardRepository struct{}

type NewBoard struct {
	Name       string
	TaskPrefix string
	CreatedBy  string
	At         time.Time
}

const boardColumns = `id, token, name, task_prefix, created_by, created_at, updated_at`

func (BoardRepository) Create(ctx context.Context, db platform.DBTX, newBoard NewBoard) (Board, error) {
	board, err := scanBoard(db.QueryRow(ctx, `
		INSERT INTO boards (token, name, task_prefix, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $5)
		RETURNING `+boardColumns,
		platform.NewPublicToken(), newBoard.Name, newBoard.TaskPrefix, newBoard.CreatedBy, newBoard.At))
	if platform.IsUniqueViolation(err) {
		return Board{}, ErrTaskPrefixTaken
	}
	if err != nil {
		return Board{}, fmt.Errorf("create board: %w", err)
	}
	return board, nil
}

func (BoardRepository) FindByToken(ctx context.Context, db platform.DBTX, token string) (Board, error) {
	return scanSingleBoard(db.QueryRow(ctx, `SELECT `+boardColumns+` FROM boards WHERE token = $1 AND deleted_at IS NULL`, token))
}

func (BoardRepository) FindByID(ctx context.Context, db platform.DBTX, id int64) (Board, error) {
	return scanSingleBoard(db.QueryRow(ctx, `SELECT `+boardColumns+` FROM boards WHERE id = $1 AND deleted_at IS NULL`, id))
}

func (BoardRepository) List(ctx context.Context, db platform.DBTX) ([]Board, error) {
	rows, err := db.Query(ctx, `SELECT `+boardColumns+` FROM boards WHERE deleted_at IS NULL ORDER BY lower(name), id`)
	if err != nil {
		return nil, fmt.Errorf("list boards: %w", err)
	}
	boards, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Board, error) {
		return scanBoard(row)
	})
	if err != nil {
		return nil, fmt.Errorf("list boards: %w", err)
	}
	return boards, nil
}

func (BoardRepository) Update(ctx context.Context, db platform.DBTX, board Board, at time.Time) (Board, error) {
	updated, err := scanSingleBoard(db.QueryRow(ctx, `
		UPDATE boards SET name = $2, task_prefix = $3, updated_at = $4
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING `+boardColumns, board.ID, board.Name, board.TaskPrefix, at))
	if platform.IsUniqueViolation(err) {
		return Board{}, ErrTaskPrefixTaken
	}
	return updated, err
}

// ListTaskPrefixesStartingWith lists the prefixes active boards use that begin with start.
func (BoardRepository) ListTaskPrefixesStartingWith(ctx context.Context, db platform.DBTX, start string) ([]string, error) {
	rows, err := db.Query(ctx, `
		SELECT task_prefix FROM boards
		WHERE deleted_at IS NULL AND starts_with(task_prefix, $1)`, start)
	if err != nil {
		return nil, fmt.Errorf("list task prefixes: %w", err)
	}
	prefixes, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("list task prefixes: %w", err)
	}
	return prefixes, nil
}

// TakeNextTaskNumber hands out the board's next task number; numbers are never reused, even after a task is deleted.
func (BoardRepository) TakeNextTaskNumber(ctx context.Context, db platform.DBTX, boardID int64) (int, error) {
	var number int
	err := db.QueryRow(ctx, `
		UPDATE boards SET next_task_number = next_task_number + 1
		WHERE id = $1
		RETURNING next_task_number - 1`, boardID).Scan(&number)
	if err != nil {
		return 0, fmt.Errorf("take next task number: %w", err)
	}
	return number, nil
}

func (BoardRepository) SoftDelete(ctx context.Context, db platform.DBTX, id int64, at time.Time) error {
	if _, err := db.Exec(ctx, `UPDATE boards SET deleted_at = $2 WHERE id = $1 AND deleted_at IS NULL`, id, at); err != nil {
		return fmt.Errorf("delete board: %w", err)
	}
	return nil
}

func scanSingleBoard(row pgx.Row) (Board, error) {
	board, err := scanBoard(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Board{}, ErrBoardNotFound
	}
	if err != nil {
		return Board{}, fmt.Errorf("find board: %w", err)
	}
	return board, nil
}

func scanBoard(row pgx.Row) (Board, error) {
	var board Board
	err := row.Scan(&board.ID, &board.Token, &board.Name, &board.TaskPrefix, &board.CreatedBy, &board.CreatedAt, &board.UpdatedAt)
	return board, err
}
