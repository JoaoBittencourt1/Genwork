package githubsync

import (
	"context"
	"fmt"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/boards"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
	"github.com/jackc/pgx/v5"
)

type BoardGithubSettingsRepository struct{}

// The merged column falls back to the board's first done preset column only while no settings row exists.
const effectiveSettingsQuery = `
SELECT boards.id, settings.id IS NOT NULL,
       opened.id, opened.token, opened.position,
       review.id, review.token, review.position,
       merged.id, merged.token, merged.position,
       coalesce(settings.final_branches, $2::text[])
FROM boards
LEFT JOIN board_github_settings settings ON settings.board_id = boards.id
LEFT JOIN board_columns opened ON opened.id = settings.opened_column_id AND opened.deleted_at IS NULL
LEFT JOIN board_columns review ON review.id = settings.review_requested_column_id AND review.deleted_at IS NULL
LEFT JOIN LATERAL (
    SELECT board_columns.id, board_columns.token, board_columns.position
    FROM board_columns
    WHERE board_columns.deleted_at IS NULL
      AND CASE WHEN settings.id IS NULL THEN board_columns.board_id = boards.id AND board_columns.preset = $3
               ELSE board_columns.id = settings.merged_column_id END
    ORDER BY board_columns.position
    LIMIT 1
) merged ON true
WHERE boards.id = ANY($1) AND boards.deleted_at IS NULL`

func (repository BoardGithubSettingsRepository) FindByBoard(ctx context.Context, db platform.DBTX, boardID int64) (BoardGithubSettings, error) {
	byBoard, err := repository.ListByBoards(ctx, db, []int64{boardID})
	if err != nil {
		return BoardGithubSettings{}, err
	}
	settings, found := byBoard[boardID]
	if !found {
		return BoardGithubSettings{}, boards.ErrBoardNotFound
	}
	return settings, nil
}

// ListByBoards loads the effective settings of many boards in one query (rule 21); deleted boards are left out.
func (BoardGithubSettingsRepository) ListByBoards(ctx context.Context, db platform.DBTX, boardIDs []int64) (map[int64]BoardGithubSettings, error) {
	rows, err := db.Query(ctx, effectiveSettingsQuery, boardIDs, DefaultFinalBranches, boards.PresetDone)
	if err != nil {
		return nil, fmt.Errorf("list board github settings: %w", err)
	}
	listed, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (BoardGithubSettings, error) {
		return scanBoardGithubSettings(row)
	})
	if err != nil {
		return nil, fmt.Errorf("list board github settings: %w", err)
	}
	byBoard := make(map[int64]BoardGithubSettings, len(listed))
	for _, settings := range listed {
		byBoard[settings.BoardID] = settings
	}
	return byBoard, nil
}

func (BoardGithubSettingsRepository) Save(ctx context.Context, db platform.DBTX, settings SavedBoardGithubSettings) error {
	_, err := db.Exec(ctx, `
		INSERT INTO board_github_settings (token, board_id, opened_column_id, review_requested_column_id, merged_column_id, final_branches,
		                                   created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
		ON CONFLICT (board_id) DO UPDATE SET
			opened_column_id = EXCLUDED.opened_column_id,
			review_requested_column_id = EXCLUDED.review_requested_column_id,
			merged_column_id = EXCLUDED.merged_column_id,
			final_branches = EXCLUDED.final_branches,
			updated_at = EXCLUDED.updated_at`,
		platform.NewPublicToken(), settings.BoardID, settings.OpenedColumnID, settings.ReviewRequestedColumnID, settings.MergedColumnID,
		settings.FinalBranches, settings.At)
	if err != nil {
		return fmt.Errorf("save board github settings: %w", err)
	}
	return nil
}

type nullableColumn struct {
	id       *int64
	token    *string
	position *int
}

func (column nullableColumn) toMapped() *MappedColumn {
	if column.id == nil {
		return nil
	}
	return &MappedColumn{ID: *column.id, Token: *column.token, Position: *column.position}
}

func scanBoardGithubSettings(row pgx.Row) (BoardGithubSettings, error) {
	var settings BoardGithubSettings
	var opened, review, merged nullableColumn
	err := row.Scan(&settings.BoardID, &settings.Saved,
		&opened.id, &opened.token, &opened.position,
		&review.id, &review.token, &review.position,
		&merged.id, &merged.token, &merged.position,
		&settings.FinalBranches)
	settings.Opened, settings.ReviewRequested, settings.Merged = opened.toMapped(), review.toMapped(), merged.toMapped()
	return settings, err
}
