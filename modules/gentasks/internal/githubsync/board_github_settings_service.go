package githubsync

import (
	"context"
	"errors"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/boards"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BoardGithubSettingsService struct {
	db       *pgxpool.Pool
	boards   boards.BoardRepository
	columns  boards.ColumnRepository
	settings BoardGithubSettingsRepository
	now      func() time.Time
}

func NewBoardGithubSettingsService(db *pgxpool.Pool, now func() time.Time) *BoardGithubSettingsService {
	return &BoardGithubSettingsService{db: db, now: now}
}

func (service *BoardGithubSettingsService) FindSettings(ctx context.Context, boardToken string) (BoardGithubSettings, error) {
	board, err := service.boards.FindByToken(ctx, service.db, boardToken)
	if err != nil {
		return BoardGithubSettings{}, err
	}
	return service.settings.FindByBoard(ctx, service.db, board.ID)
}

// UpdateSettings saves the effective settings with the changes applied, so the first save keeps every default it did not change.
func (service *BoardGithubSettingsService) UpdateSettings(ctx context.Context, boardToken string, changes BoardGithubSettingsChanges) (BoardGithubSettings, error) {
	var updated BoardGithubSettings
	err := pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		board, err := service.boards.FindByToken(ctx, tx, boardToken)
		if err != nil {
			return err
		}
		if err := platform.LockBoard(ctx, tx, board.ID); err != nil {
			return err
		}
		current, err := service.settings.FindByBoard(ctx, tx, board.ID)
		if err != nil || changes.IsEmpty() {
			updated = current
			return err
		}
		next, err := service.applyChanges(ctx, tx, current, changes)
		if err != nil {
			return err
		}
		if err := service.settings.Save(ctx, tx, next.ToSaved(service.now())); err != nil {
			return err
		}
		updated, err = service.settings.FindByBoard(ctx, tx, board.ID)
		return err
	})
	return updated, err
}

func (service *BoardGithubSettingsService) applyChanges(ctx context.Context, tx pgx.Tx, settings BoardGithubSettings, changes BoardGithubSettingsChanges) (BoardGithubSettings, error) {
	fields := map[string]string{}
	mappings := []struct {
		field  string
		change ColumnMappingChange
		target **MappedColumn
	}{
		{fieldOpenedColumn, changes.Opened, &settings.Opened},
		{fieldReviewColumn, changes.ReviewRequested, &settings.ReviewRequested},
		{fieldMergedColumn, changes.Merged, &settings.Merged},
	}
	for _, mapping := range mappings {
		if !mapping.change.Present {
			continue
		}
		column, err := service.findBoardColumn(ctx, tx, settings.BoardID, mapping.change.Token)
		if errors.Is(err, boards.ErrColumnNotFound) {
			fields[mapping.field] = "invalid"
			continue
		}
		if err != nil {
			return BoardGithubSettings{}, err
		}
		*mapping.target = column
	}
	if len(fields) > 0 {
		return BoardGithubSettings{}, httpapi.NewValidationError(fields)
	}
	if changes.FinalBranches != nil {
		settings.FinalBranches = changes.FinalBranches
	}
	return settings, nil
}

// findBoardColumn returns nil for a cleared mapping, and ErrColumnNotFound for a column of another board or a deleted one.
func (service *BoardGithubSettingsService) findBoardColumn(ctx context.Context, tx pgx.Tx, boardID int64, token *string) (*MappedColumn, error) {
	if token == nil {
		return nil, nil
	}
	column, err := service.columns.FindByToken(ctx, tx, *token)
	if err != nil {
		return nil, err
	}
	if column.BoardID != boardID {
		return nil, boards.ErrColumnNotFound
	}
	return &MappedColumn{ID: column.ID, Token: column.Token, Position: column.Position}, nil
}
