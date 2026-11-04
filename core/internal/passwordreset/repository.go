package passwordreset

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/jackc/pgx/v5"
)

var errNoReset = errors.New("password reset not found")

type Repository struct{}

type NewReset struct {
	UserID     int64
	SecretHash []byte
	CreatedAt  time.Time
	ExpiresAt  time.Time
}

// LockUser serializes resets per user so concurrent requests cannot both pass the interval check.
func (Repository) LockUser(ctx context.Context, tx pgx.Tx, userID int64) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('password-reset:' || $1::bigint::text))`, userID); err != nil {
		return fmt.Errorf("lock password reset user: %w", err)
	}
	return nil
}

func (Repository) HasCreatedSince(ctx context.Context, db platform.DBTX, userID int64, since time.Time) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM password_resets WHERE user_id = $1 AND created_at > $2)`,
		userID, since).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check recent password reset: %w", err)
	}
	return exists, nil
}

func (Repository) Create(ctx context.Context, db platform.DBTX, newReset NewReset) error {
	_, err := db.Exec(ctx, `
		INSERT INTO password_resets (user_id, secret_hash, created_at, expires_at)
		VALUES ($1, $2, $3, $4)`,
		newReset.UserID, newReset.SecretHash, newReset.CreatedAt, newReset.ExpiresAt)
	if err != nil {
		return fmt.Errorf("create password reset: %w", err)
	}
	return nil
}

// LockPendingBySecret re-evaluates the pending condition after waiting for the row lock, so a concurrent use finds nothing.
func (Repository) LockPendingBySecret(ctx context.Context, tx pgx.Tx, secretHash []byte, now time.Time) (Reset, error) {
	var reset Reset
	err := tx.QueryRow(ctx, `
		SELECT id, user_id, created_at, expires_at FROM password_resets
		WHERE secret_hash = $1 AND used_at IS NULL AND expires_at > $2
		FOR UPDATE`, secretHash, now).Scan(&reset.ID, &reset.UserID, &reset.CreatedAt, &reset.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Reset{}, errNoReset
	}
	if err != nil {
		return Reset{}, fmt.Errorf("find password reset: %w", err)
	}
	return reset, nil
}

func (Repository) MarkPendingUsedForUser(ctx context.Context, db platform.DBTX, userID int64, now time.Time) error {
	if _, err := db.Exec(ctx, `UPDATE password_resets SET used_at = $2 WHERE user_id = $1 AND used_at IS NULL`, userID, now); err != nil {
		return fmt.Errorf("invalidate password resets: %w", err)
	}
	return nil
}
