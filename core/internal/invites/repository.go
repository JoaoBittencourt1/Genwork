package invites

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/jackc/pgx/v5"
)

var errNoInvite = errors.New("invite not found")

type Repository struct{}

type NewInvite struct {
	Email           string
	RoleID          int64
	InvitedByUserID int64
	SecretHash      []byte
	CreatedAt       time.Time
	ExpiresAt       time.Time
}

const inviteSelect = `
SELECT invites.id, invites.token, invites.email, roles.id, roles.token, roles.name,
       inviter.id, inviter.token, inviter.name,
       invites.created_at, invites.expires_at, invites.accepted_at, invites.revoked_at
FROM invites
JOIN roles ON roles.id = invites.role_id
JOIN users AS inviter ON inviter.id = invites.invited_by_user_id`

const pendingCondition = `invites.accepted_at IS NULL AND invites.revoked_at IS NULL AND invites.expires_at > $2`

// LockEmail serializes invite creation per e-mail so two concurrent requests cannot both pass the pending check.
func (Repository) LockEmail(ctx context.Context, tx pgx.Tx, normalizedEmail string) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('invite:' || $1))`, normalizedEmail); err != nil {
		return fmt.Errorf("lock invite e-mail: %w", err)
	}
	return nil
}

func (Repository) HasPendingForEmail(ctx context.Context, db platform.DBTX, normalizedEmail string, now time.Time) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM invites WHERE invites.email = $1 AND `+pendingCondition+`)`,
		normalizedEmail, now).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check pending invite: %w", err)
	}
	return exists, nil
}

func (repository Repository) Create(ctx context.Context, db platform.DBTX, newInvite NewInvite) (Invite, error) {
	var inviteID int64
	err := db.QueryRow(ctx, `
		INSERT INTO invites (email, role_id, invited_by_user_id, secret_hash, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`,
		newInvite.Email, newInvite.RoleID, newInvite.InvitedByUserID, newInvite.SecretHash,
		newInvite.CreatedAt, newInvite.ExpiresAt).Scan(&inviteID)
	if err != nil {
		return Invite{}, fmt.Errorf("create invite: %w", err)
	}
	return repository.findOne(ctx, db, ` WHERE invites.id = $1`, inviteID)
}

func (repository Repository) FindByToken(ctx context.Context, db platform.DBTX, token string) (Invite, error) {
	return repository.findOne(ctx, db, ` WHERE invites.token = $1`, token)
}

func (repository Repository) FindPendingBySecret(ctx context.Context, db platform.DBTX, secretHash []byte, now time.Time) (Invite, error) {
	return repository.findOne(ctx, db, ` WHERE invites.secret_hash = $1 AND `+pendingCondition, secretHash, now)
}

// LockPendingBySecret re-evaluates the pending condition after waiting for the row lock, so a concurrent acceptance finds nothing.
func (repository Repository) LockPendingBySecret(ctx context.Context, tx pgx.Tx, secretHash []byte, now time.Time) (Invite, error) {
	return repository.findOne(ctx, tx, ` WHERE invites.secret_hash = $1 AND `+pendingCondition+` FOR UPDATE OF invites`, secretHash, now)
}

func (Repository) List(ctx context.Context, db platform.DBTX) ([]Invite, error) {
	rows, err := db.Query(ctx, inviteSelect+` ORDER BY invites.created_at DESC, invites.id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list invites: %w", err)
	}
	invites, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Invite, error) {
		return scanInvite(row)
	})
	if err != nil {
		return nil, fmt.Errorf("list invites: %w", err)
	}
	return invites, nil
}

func (Repository) Revoke(ctx context.Context, db platform.DBTX, inviteID int64, now time.Time) error {
	if _, err := db.Exec(ctx, `UPDATE invites SET revoked_at = $2 WHERE id = $1`, inviteID, now); err != nil {
		return fmt.Errorf("revoke invite: %w", err)
	}
	return nil
}

func (Repository) ReplaceSecret(ctx context.Context, db platform.DBTX, inviteID int64, secretHash []byte, expiresAt time.Time) error {
	if _, err := db.Exec(ctx, `UPDATE invites SET secret_hash = $2, expires_at = $3 WHERE id = $1`, inviteID, secretHash, expiresAt); err != nil {
		return fmt.Errorf("replace invite secret: %w", err)
	}
	return nil
}

func (Repository) MarkAccepted(ctx context.Context, db platform.DBTX, inviteID, userID int64, now time.Time) error {
	if _, err := db.Exec(ctx, `UPDATE invites SET accepted_at = $3, accepted_user_id = $2 WHERE id = $1`, inviteID, userID, now); err != nil {
		return fmt.Errorf("accept invite: %w", err)
	}
	return nil
}

func (Repository) findOne(ctx context.Context, db platform.DBTX, condition string, arguments ...any) (Invite, error) {
	invite, err := scanInvite(db.QueryRow(ctx, inviteSelect+condition, arguments...))
	if errors.Is(err, pgx.ErrNoRows) {
		return Invite{}, errNoInvite
	}
	if err != nil {
		return Invite{}, fmt.Errorf("find invite: %w", err)
	}
	return invite, nil
}

func scanInvite(row pgx.Row) (Invite, error) {
	var invite Invite
	err := row.Scan(&invite.ID, &invite.Token, &invite.Email, &invite.RoleID, &invite.RoleToken, &invite.RoleName,
		&invite.InvitedByUserID, &invite.InviterToken, &invite.InviterName,
		&invite.CreatedAt, &invite.ExpiresAt, &invite.AcceptedAt, &invite.RevokedAt)
	return invite, err
}
