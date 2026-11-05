package accounts

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform"
	"github.com/jackc/pgx/v5"
)

type AiAccountRepository struct{}

type NewAiAccount struct {
	Token            string
	Name             string
	Provider         Provider
	Mode             AccountMode
	RunnerID         *int64
	APIKeyCiphertext []byte
	APIKeyLast4      *string
	Status           AccountStatus
	At               time.Time
}

const accountSelect = `
SELECT ai_accounts.id, ai_accounts.token, ai_accounts.name, ai_accounts.provider, ai_accounts.mode, ai_accounts.runner_id,
       runners.token, ai_accounts.api_key_ciphertext, ai_accounts.api_key_last4, ai_accounts.status, ai_accounts.last_error,
       ai_accounts.created_at, ai_accounts.updated_at
FROM ai_accounts
LEFT JOIN runners ON runners.id = ai_accounts.runner_id`

func (repository AiAccountRepository) Create(ctx context.Context, db platform.DBTX, newAccount NewAiAccount) (AiAccount, error) {
	var id int64
	err := db.QueryRow(ctx, `
		INSERT INTO ai_accounts (token, name, provider, mode, runner_id, api_key_ciphertext, api_key_last4, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
		RETURNING id`,
		newAccount.Token, newAccount.Name, newAccount.Provider, newAccount.Mode, newAccount.RunnerID,
		newAccount.APIKeyCiphertext, newAccount.APIKeyLast4, newAccount.Status, newAccount.At).Scan(&id)
	if platform.IsUniqueViolation(err) {
		return AiAccount{}, ErrAccountNameTaken
	}
	if err != nil {
		return AiAccount{}, fmt.Errorf("create account: %w", err)
	}
	return repository.FindByID(ctx, db, id)
}

func (AiAccountRepository) FindByID(ctx context.Context, db platform.DBTX, id int64) (AiAccount, error) {
	return scanSingleAccount(db.QueryRow(ctx, accountSelect+` WHERE ai_accounts.id = $1 AND ai_accounts.deleted_at IS NULL`, id))
}

func (AiAccountRepository) FindByToken(ctx context.Context, db platform.DBTX, token string) (AiAccount, error) {
	return scanSingleAccount(db.QueryRow(ctx, accountSelect+` WHERE ai_accounts.token = $1 AND ai_accounts.deleted_at IS NULL`, token))
}

func (AiAccountRepository) List(ctx context.Context, db platform.DBTX) ([]AiAccount, error) {
	rows, err := db.Query(ctx, accountSelect+` WHERE ai_accounts.deleted_at IS NULL ORDER BY ai_accounts.name`)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	listed, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (AiAccount, error) { return scanAccount(row) })
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	return listed, nil
}

func (AiAccountRepository) UpdateName(ctx context.Context, db platform.DBTX, id int64, name string, at time.Time) error {
	tag, err := db.Exec(ctx, `UPDATE ai_accounts SET name = $2, updated_at = $3 WHERE id = $1 AND deleted_at IS NULL`, id, name, at)
	if platform.IsUniqueViolation(err) {
		return ErrAccountNameTaken
	}
	if err != nil {
		return fmt.Errorf("rename account: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrAccountNotFound
	}
	return nil
}

// ReplaceAPIKey also returns the account to ready, since a new key is the fix for an authentication error.
func (AiAccountRepository) ReplaceAPIKey(ctx context.Context, db platform.DBTX, id int64, ciphertext []byte, last4 string, at time.Time) error {
	tag, err := db.Exec(ctx, `
		UPDATE ai_accounts SET api_key_ciphertext = $2, api_key_last4 = $3, status = 'ready', last_error = NULL, updated_at = $4
		WHERE id = $1 AND deleted_at IS NULL AND mode = 'api_key'`, id, ciphertext, last4, at)
	if err != nil {
		return fmt.Errorf("replace api key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrAccountNotFound
	}
	return nil
}

func (AiAccountRepository) UpdateStatus(ctx context.Context, db platform.DBTX, id int64, status AccountStatus, lastError *string, at time.Time) error {
	_, err := db.Exec(ctx, `UPDATE ai_accounts SET status = $2, last_error = $3, updated_at = $4 WHERE id = $1 AND deleted_at IS NULL`,
		id, status, lastError, at)
	if err != nil {
		return fmt.Errorf("update account status: %w", err)
	}
	return nil
}

// SoftDelete clears the key ciphertext in the same statement, so a secret never outlives its record (rule 67).
func (AiAccountRepository) SoftDelete(ctx context.Context, db platform.DBTX, id int64, at time.Time) error {
	tag, err := db.Exec(ctx, `
		UPDATE ai_accounts SET deleted_at = $2, updated_at = $2, api_key_ciphertext = NULL, api_key_last4 = NULL
		WHERE id = $1 AND deleted_at IS NULL`, id, at)
	if err != nil {
		return fmt.Errorf("remove account: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrAccountNotFound
	}
	return nil
}

func scanSingleAccount(row pgx.Row) (AiAccount, error) {
	account, err := scanAccount(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return AiAccount{}, ErrAccountNotFound
	}
	if err != nil {
		return AiAccount{}, fmt.Errorf("find account: %w", err)
	}
	return account, nil
}

func scanAccount(row pgx.Row) (AiAccount, error) {
	var account AiAccount
	err := row.Scan(&account.ID, &account.Token, &account.Name, &account.Provider, &account.Mode, &account.RunnerID, &account.RunnerToken,
		&account.APIKeyCiphertext, &account.APIKeyLast4, &account.Status, &account.LastError, &account.CreatedAt, &account.UpdatedAt)
	return account, err
}
