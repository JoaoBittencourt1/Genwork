package accounts

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform"
	"github.com/jackc/pgx/v5"
)

type AccountModelRepository struct{}

const accountModelColumns = `id, token, account_id, model_id, display_name, created_at`

func (AccountModelRepository) Create(ctx context.Context, db platform.DBTX, accountID int64, modelID, displayName string, at time.Time) (AccountModel, error) {
	model, err := scanAccountModel(db.QueryRow(ctx, `
		INSERT INTO ai_account_models (token, account_id, model_id, display_name, created_at) VALUES ($1, $2, $3, $4, $5)
		RETURNING `+accountModelColumns, platform.NewPublicToken(), accountID, modelID, displayName, at))
	if platform.IsUniqueViolation(err) {
		return AccountModel{}, ErrModelAlreadyDefined
	}
	if err != nil {
		return AccountModel{}, fmt.Errorf("create account model: %w", err)
	}
	return model, nil
}

func (AccountModelRepository) ListByAccount(ctx context.Context, db platform.DBTX, accountID int64) ([]AccountModel, error) {
	rows, err := db.Query(ctx, `SELECT `+accountModelColumns+` FROM ai_account_models WHERE account_id = $1 AND deleted_at IS NULL ORDER BY model_id`, accountID)
	if err != nil {
		return nil, fmt.Errorf("list account models: %w", err)
	}
	listed, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (AccountModel, error) { return scanAccountModel(row) })
	if err != nil {
		return nil, fmt.Errorf("list account models: %w", err)
	}
	return listed, nil
}

// ListByAccounts loads the custom models of many accounts in one query (rule 21).
func (AccountModelRepository) ListByAccounts(ctx context.Context, db platform.DBTX, accountIDs []int64) (map[int64][]AccountModel, error) {
	rows, err := db.Query(ctx, `SELECT `+accountModelColumns+` FROM ai_account_models WHERE account_id = ANY($1) AND deleted_at IS NULL ORDER BY model_id`, accountIDs)
	if err != nil {
		return nil, fmt.Errorf("list account models: %w", err)
	}
	listed, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (AccountModel, error) { return scanAccountModel(row) })
	if err != nil {
		return nil, fmt.Errorf("list account models: %w", err)
	}
	byAccount := make(map[int64][]AccountModel)
	for _, model := range listed {
		byAccount[model.AccountID] = append(byAccount[model.AccountID], model)
	}
	return byAccount, nil
}

func (AccountModelRepository) SoftDeleteByToken(ctx context.Context, db platform.DBTX, accountID int64, token string, at time.Time) error {
	tag, err := db.Exec(ctx, `UPDATE ai_account_models SET deleted_at = $3 WHERE account_id = $1 AND token = $2 AND deleted_at IS NULL`, accountID, token, at)
	if err != nil {
		return fmt.Errorf("remove account model: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrModelNotFound
	}
	return nil
}

func scanAccountModel(row pgx.Row) (AccountModel, error) {
	var model AccountModel
	err := row.Scan(&model.ID, &model.Token, &model.AccountID, &model.ModelID, &model.DisplayName, &model.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return AccountModel{}, ErrModelNotFound
	}
	return model, err
}
