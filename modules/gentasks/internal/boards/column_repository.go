package boards

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
	"github.com/jackc/pgx/v5"
)

type ColumnRepository struct{}

type NewColumn struct {
	BoardID  int64
	Preset   *ColumnPreset
	Name     *string
	Position int
	At       time.Time
}

const columnColumns = `board_columns.id, board_columns.token, board_columns.board_id, board_columns.preset, board_columns.name,
	board_columns.position, board_columns.created_at, board_columns.updated_at`

func (ColumnRepository) Create(ctx context.Context, db platform.DBTX, newColumn NewColumn) (Column, error) {
	column, err := scanColumn(db.QueryRow(ctx, `
		INSERT INTO board_columns (token, board_id, preset, name, position, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		RETURNING `+columnColumns,
		platform.NewPublicToken(), newColumn.BoardID, newColumn.Preset, newColumn.Name, newColumn.Position, newColumn.At))
	if err != nil {
		return Column{}, fmt.Errorf("create column: %w", err)
	}
	return column, nil
}

// FindByToken only finds columns whose board still exists.
func (ColumnRepository) FindByToken(ctx context.Context, db platform.DBTX, token string) (Column, error) {
	return scanSingleColumn(db.QueryRow(ctx, `
		SELECT `+columnColumns+`
		FROM board_columns JOIN boards ON boards.id = board_columns.board_id
		WHERE board_columns.token = $1 AND board_columns.deleted_at IS NULL AND boards.deleted_at IS NULL`, token))
}

func (ColumnRepository) FindByID(ctx context.Context, db platform.DBTX, id int64) (Column, error) {
	return scanSingleColumn(db.QueryRow(ctx, `SELECT `+columnColumns+` FROM board_columns WHERE id = $1 AND deleted_at IS NULL`, id))
}

func (ColumnRepository) FindFirstInBoard(ctx context.Context, db platform.DBTX, boardID int64) (Column, error) {
	return scanSingleColumn(db.QueryRow(ctx, `
		SELECT `+columnColumns+` FROM board_columns
		WHERE board_id = $1 AND deleted_at IS NULL
		ORDER BY position LIMIT 1`, boardID))
}

func (ColumnRepository) ListByBoard(ctx context.Context, db platform.DBTX, boardID int64) ([]Column, error) {
	rows, err := db.Query(ctx, `
		SELECT `+columnColumns+` FROM board_columns
		WHERE board_id = $1 AND deleted_at IS NULL
		ORDER BY position`, boardID)
	if err != nil {
		return nil, fmt.Errorf("list columns: %w", err)
	}
	columns, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Column, error) {
		return scanColumn(row)
	})
	if err != nil {
		return nil, fmt.Errorf("list columns: %w", err)
	}
	return columns, nil
}

func (ColumnRepository) CountInBoard(ctx context.Context, db platform.DBTX, boardID int64) (int, error) {
	var count int
	err := db.QueryRow(ctx, `SELECT count(*) FROM board_columns WHERE board_id = $1 AND deleted_at IS NULL`, boardID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count columns: %w", err)
	}
	return count, nil
}

func (ColumnRepository) Rename(ctx context.Context, db platform.DBTX, id int64, name *string, at time.Time) (Column, error) {
	return scanSingleColumn(db.QueryRow(ctx, `
		UPDATE board_columns SET name = $2, updated_at = $3
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING `+columnColumns, id, name, at))
}

// ShiftPositions moves every column positioned in [from, to] of the board by delta.
func (ColumnRepository) ShiftPositions(ctx context.Context, db platform.DBTX, boardID int64, from, to, delta int) error {
	_, err := db.Exec(ctx, `
		UPDATE board_columns SET position = position + $4
		WHERE board_id = $1 AND deleted_at IS NULL AND position BETWEEN $2 AND $3`, boardID, from, to, delta)
	if err != nil {
		return fmt.Errorf("shift column positions: %w", err)
	}
	return nil
}

func (ColumnRepository) SetPosition(ctx context.Context, db platform.DBTX, id int64, position int, at time.Time) error {
	if _, err := db.Exec(ctx, `UPDATE board_columns SET position = $2, updated_at = $3 WHERE id = $1`, id, position, at); err != nil {
		return fmt.Errorf("set column position: %w", err)
	}
	return nil
}

func (ColumnRepository) SoftDelete(ctx context.Context, db platform.DBTX, id int64, at time.Time) error {
	if _, err := db.Exec(ctx, `UPDATE board_columns SET deleted_at = $2 WHERE id = $1 AND deleted_at IS NULL`, id, at); err != nil {
		return fmt.Errorf("delete column: %w", err)
	}
	return nil
}

func scanSingleColumn(row pgx.Row) (Column, error) {
	column, err := scanColumn(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Column{}, ErrColumnNotFound
	}
	if err != nil {
		return Column{}, fmt.Errorf("find column: %w", err)
	}
	return column, nil
}

func scanColumn(row pgx.Row) (Column, error) {
	var column Column
	err := row.Scan(&column.ID, &column.Token, &column.BoardID, &column.Preset, &column.Name,
		&column.Position, &column.CreatedAt, &column.UpdatedAt)
	return column, err
}
