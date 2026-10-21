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
	Name      string
	CreatedBy string
	At        time.Time
}

const boardColumns = `id, token, name, created_by, created_at, updated_at`

func (BoardRepository) Create(ctx context.Context, db platform.DBTX, newBoard NewBoard) (Board, error) {
	board, err := scanBoard(db.QueryRow(ctx, `
		INSERT INTO boards (token, name, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $4)
		RETURNING `+boardColumns,
		platform.NewPublicToken(), newBoard.Name, newBoard.CreatedBy, newBoard.At))
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

func (BoardRepository) Rename(ctx context.Context, db platform.DBTX, id int64, name string, at time.Time) (Board, error) {
	return scanSingleBoard(db.QueryRow(ctx, `
		UPDATE boards SET name = $2, updated_at = $3
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING `+boardColumns, id, name, at))
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
	err := row.Scan(&board.ID, &board.Token, &board.Name, &board.CreatedBy, &board.CreatedAt, &board.UpdatedAt)
	return board, err
}
