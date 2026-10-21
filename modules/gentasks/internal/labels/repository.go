package labels

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
	"github.com/jackc/pgx/v5"
)

type LabelRepository struct{}

type NewLabel struct {
	BoardID int64
	Name    string
	Color   LabelColor
	At      time.Time
}

const labelColumns = `labels.id, labels.token, labels.board_id, labels.name, labels.color, labels.created_at, labels.updated_at`

func (LabelRepository) Create(ctx context.Context, db platform.DBTX, newLabel NewLabel) (Label, error) {
	label, err := scanLabel(db.QueryRow(ctx, `
		INSERT INTO labels (token, board_id, name, color, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $5)
		RETURNING `+labelColumns,
		platform.NewPublicToken(), newLabel.BoardID, newLabel.Name, newLabel.Color, newLabel.At))
	if platform.IsUniqueViolation(err) {
		return Label{}, ErrLabelNameTaken
	}
	if err != nil {
		return Label{}, fmt.Errorf("create label: %w", err)
	}
	return label, nil
}

// FindByToken only finds labels whose board still exists.
func (LabelRepository) FindByToken(ctx context.Context, db platform.DBTX, token string) (Label, error) {
	label, err := scanLabel(db.QueryRow(ctx, `
		SELECT `+labelColumns+`
		FROM labels JOIN boards ON boards.id = labels.board_id
		WHERE labels.token = $1 AND labels.deleted_at IS NULL AND boards.deleted_at IS NULL`, token))
	if errors.Is(err, pgx.ErrNoRows) {
		return Label{}, ErrLabelNotFound
	}
	if err != nil {
		return Label{}, fmt.Errorf("find label: %w", err)
	}
	return label, nil
}

func (LabelRepository) ListByBoard(ctx context.Context, db platform.DBTX, boardID int64) ([]Label, error) {
	return listLabels(ctx, db, `
		SELECT `+labelColumns+` FROM labels
		WHERE board_id = $1 AND deleted_at IS NULL
		ORDER BY lower(name), id`, boardID)
}

// FindInBoardByTokens returns the board's active labels among tokens; callers compare lengths to spot foreign or deleted ones.
func (LabelRepository) FindInBoardByTokens(ctx context.Context, db platform.DBTX, boardID int64, tokens []string) ([]Label, error) {
	return listLabels(ctx, db, `
		SELECT `+labelColumns+` FROM labels
		WHERE board_id = $1 AND token = ANY($2) AND deleted_at IS NULL
		ORDER BY lower(name), id`, boardID, tokens)
}

func (LabelRepository) Update(ctx context.Context, db platform.DBTX, label Label, at time.Time) (Label, error) {
	updated, err := scanLabel(db.QueryRow(ctx, `
		UPDATE labels SET name = $2, color = $3, updated_at = $4
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING `+labelColumns, label.ID, label.Name, label.Color, at))
	if platform.IsUniqueViolation(err) {
		return Label{}, ErrLabelNameTaken
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return Label{}, ErrLabelNotFound
	}
	if err != nil {
		return Label{}, fmt.Errorf("update label: %w", err)
	}
	return updated, nil
}

func (LabelRepository) SoftDelete(ctx context.Context, db platform.DBTX, id int64, at time.Time) error {
	if _, err := db.Exec(ctx, `UPDATE labels SET deleted_at = $2 WHERE id = $1 AND deleted_at IS NULL`, id, at); err != nil {
		return fmt.Errorf("delete label: %w", err)
	}
	return nil
}

func listLabels(ctx context.Context, db platform.DBTX, query string, arguments ...any) ([]Label, error) {
	rows, err := db.Query(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("list labels: %w", err)
	}
	labels, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Label, error) {
		return scanLabel(row)
	})
	if err != nil {
		return nil, fmt.Errorf("list labels: %w", err)
	}
	return labels, nil
}

func scanLabel(row pgx.Row) (Label, error) {
	var label Label
	err := row.Scan(&label.ID, &label.Token, &label.BoardID, &label.Name, &label.Color, &label.CreatedAt, &label.UpdatedAt)
	return label, err
}
