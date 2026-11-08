package accounts

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform"
	"github.com/jackc/pgx/v5"
)

type AccountSignInRepository struct{}

const signInColumns = `account_sign_ins.id, account_sign_ins.token, account_sign_ins.account_id, account_sign_ins.state,
       account_sign_ins.sign_in_url, account_sign_ins.user_code, account_sign_ins.expects_code, account_sign_ins.code_ciphertext,
       account_sign_ins.failure, account_sign_ins.expires_at, account_sign_ins.created_at, account_sign_ins.updated_at`

const activeSignInStates = `('requested', 'awaiting_user', 'code_submitted', 'verifying')`

func (repository AccountSignInRepository) Create(ctx context.Context, db platform.DBTX, token string, accountID int64, expiresAt, at time.Time) (AccountSignIn, error) {
	signIn, err := scanSignIn(db.QueryRow(ctx, `
		INSERT INTO account_sign_ins (token, account_id, state, expires_at, created_at, updated_at)
		VALUES ($1, $2, 'requested', $3, $4, $4)
		RETURNING `+signInColumns, token, accountID, expiresAt, at))
	if platform.IsUniqueViolation(err) {
		return AccountSignIn{}, ErrSignInInFlight
	}
	if err != nil {
		return AccountSignIn{}, fmt.Errorf("create sign-in: %w", err)
	}
	return signIn, nil
}

func (AccountSignInRepository) FindByToken(ctx context.Context, db platform.DBTX, accountID int64, token string) (AccountSignIn, error) {
	signIn, err := scanSignIn(db.QueryRow(ctx, `SELECT `+signInColumns+` FROM account_sign_ins WHERE token = $1 AND account_id = $2`, token, accountID))
	if errors.Is(err, pgx.ErrNoRows) {
		return AccountSignIn{}, ErrSignInNotFound
	}
	if err != nil {
		return AccountSignIn{}, fmt.Errorf("find sign-in: %w", err)
	}
	return signIn, nil
}

// FindForRunner finds a sign-in only when the runner holds its account, so one runner cannot drive another's sign-ins.
func (AccountSignInRepository) FindForRunner(ctx context.Context, db platform.DBTX, runnerID int64, token string) (AccountSignIn, error) {
	signIn, err := scanSignIn(db.QueryRow(ctx, `
		SELECT `+signInColumns+` FROM account_sign_ins
		JOIN ai_accounts ON ai_accounts.id = account_sign_ins.account_id
		WHERE account_sign_ins.token = $1 AND ai_accounts.runner_id = $2 AND ai_accounts.deleted_at IS NULL
		FOR UPDATE OF account_sign_ins`, token, runnerID))
	if errors.Is(err, pgx.ErrNoRows) {
		return AccountSignIn{}, ErrSignInNotFound
	}
	if err != nil {
		return AccountSignIn{}, fmt.Errorf("find sign-in: %w", err)
	}
	return signIn, nil
}

// RunnerSignInRow is an active sign-in with the account fields the runner needs.
type RunnerSignInRow struct {
	SignIn       AccountSignIn
	AccountToken string
	Provider     Provider
}

func (AccountSignInRepository) ListActiveForRunner(ctx context.Context, db platform.DBTX, runnerID int64) ([]RunnerSignInRow, error) {
	rows, err := db.Query(ctx, `
		SELECT `+signInColumns+`, ai_accounts.token, ai_accounts.provider FROM account_sign_ins
		JOIN ai_accounts ON ai_accounts.id = account_sign_ins.account_id
		WHERE ai_accounts.runner_id = $1 AND ai_accounts.deleted_at IS NULL AND account_sign_ins.state IN `+activeSignInStates+`
		ORDER BY account_sign_ins.created_at`, runnerID)
	if err != nil {
		return nil, fmt.Errorf("list runner sign-ins: %w", err)
	}
	listed, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (RunnerSignInRow, error) {
		var entry RunnerSignInRow
		signIn := &entry.SignIn
		err := row.Scan(&signIn.ID, &signIn.Token, &signIn.AccountID, &signIn.State, &signIn.SignInURL, &signIn.UserCode, &signIn.ExpectsCode,
			&signIn.CodeCiphertext, &signIn.Failure, &signIn.ExpiresAt, &signIn.CreatedAt, &signIn.UpdatedAt, &entry.AccountToken, &entry.Provider)
		return entry, err
	})
	if err != nil {
		return nil, fmt.Errorf("list runner sign-ins: %w", err)
	}
	return listed, nil
}

// ListLatestByAccounts returns each account's most recent sign-in in one query (rule 21).
func (AccountSignInRepository) ListLatestByAccounts(ctx context.Context, db platform.DBTX, accountIDs []int64) (map[int64]AccountSignIn, error) {
	latest := map[int64]AccountSignIn{}
	if len(accountIDs) == 0 {
		return latest, nil
	}
	rows, err := db.Query(ctx, `
		SELECT DISTINCT ON (account_id) `+signInColumns+` FROM account_sign_ins
		WHERE account_id = ANY($1) ORDER BY account_id, created_at DESC, id DESC`, accountIDs)
	if err != nil {
		return nil, fmt.Errorf("list latest sign-ins: %w", err)
	}
	listed, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (AccountSignIn, error) { return scanSignIn(row) })
	if err != nil {
		return nil, fmt.Errorf("list latest sign-ins: %w", err)
	}
	for _, signIn := range listed {
		latest[signIn.AccountID] = signIn
	}
	return latest, nil
}

// CancelActive ends the account's active sign-in, dropping any pasted code with it.
func (AccountSignInRepository) CancelActive(ctx context.Context, db platform.DBTX, accountID int64, at time.Time) error {
	_, err := db.Exec(ctx, `
		UPDATE account_sign_ins SET state = 'cancelled', code_ciphertext = NULL, updated_at = $2
		WHERE account_id = $1 AND state IN `+activeSignInStates, accountID, at)
	if err != nil {
		return fmt.Errorf("cancel sign-in: %w", err)
	}
	return nil
}

// ExpireOverdue ends every active sign-in past its deadline and returns the accounts they belonged to.
func (AccountSignInRepository) ExpireOverdue(ctx context.Context, db platform.DBTX, at time.Time) ([]int64, error) {
	rows, err := db.Query(ctx, `
		UPDATE account_sign_ins SET state = 'expired', code_ciphertext = NULL, failure = $2, updated_at = $1
		WHERE state IN `+activeSignInStates+` AND expires_at <= $1
		RETURNING account_id`, at, SignInExpiredError)
	if err != nil {
		return nil, fmt.Errorf("expire sign-ins: %w", err)
	}
	accountIDs, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return nil, fmt.Errorf("expire sign-ins: %w", err)
	}
	return accountIDs, nil
}

// SignInUpdate is a sign-in's next state; nil pointers leave a column unchanged, and ClearCode drops the pasted code.
type SignInUpdate struct {
	State          SignInState
	SignInURL      *string
	UserCode       *string
	ExpectsCode    *bool
	CodeCiphertext []byte
	ClearCode      bool
	Failure        *string
	At             time.Time
}

func (AccountSignInRepository) Update(ctx context.Context, db platform.DBTX, id int64, update SignInUpdate) error {
	_, err := db.Exec(ctx, `
		UPDATE account_sign_ins SET state = $2, sign_in_url = COALESCE($3, sign_in_url), user_code = COALESCE($4, user_code),
		       expects_code = COALESCE($5, expects_code),
		       code_ciphertext = CASE WHEN $7 THEN NULL WHEN $6::bytea IS NOT NULL THEN $6 ELSE code_ciphertext END,
		       failure = COALESCE($8, failure), updated_at = $9
		WHERE id = $1`,
		id, update.State, update.SignInURL, update.UserCode, update.ExpectsCode, update.CodeCiphertext, update.ClearCode, update.Failure, update.At)
	if err != nil {
		return fmt.Errorf("update sign-in: %w", err)
	}
	return nil
}

func scanSignIn(row pgx.Row) (AccountSignIn, error) {
	var signIn AccountSignIn
	err := row.Scan(&signIn.ID, &signIn.Token, &signIn.AccountID, &signIn.State, &signIn.SignInURL, &signIn.UserCode, &signIn.ExpectsCode,
		&signIn.CodeCiphertext, &signIn.Failure, &signIn.ExpiresAt, &signIn.CreatedAt, &signIn.UpdatedAt)
	return signIn, err
}
