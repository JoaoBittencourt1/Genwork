package sessions

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/jackc/pgx/v5"
)

var (
	ErrSessionNotFound      = errors.New("session not found")
	ErrRefreshTokenNotFound = errors.New("refresh token not found")
)

type Repository struct{}

type NewSession struct {
	UserID        int64
	ClientType    ClientType
	DeviceName    *string
	OAuthClientID *int64
	Now           time.Time
}

const sessionColumns = `
	sessions.id, sessions.token, users.id, users.token, users.role_id, sessions.client_type, sessions.device_name,
	sessions.oauth_client_id, sessions.created_at, sessions.last_used_at, sessions.idle_expires_at, sessions.absolute_expires_at,
	sessions.revoked_at`

func (Repository) Create(ctx context.Context, db platform.DBTX, newSession NewSession) (Session, error) {
	absoluteExpiresAt := newSession.Now.Add(AbsoluteLifetime)
	var sessionID int64
	err := db.QueryRow(ctx, `
		INSERT INTO sessions (user_id, client_type, device_name, oauth_client_id, created_at, last_used_at, idle_expires_at, absolute_expires_at)
		VALUES ($1, $2, $3, $4, $5, $5, $6, $7)
		RETURNING id`,
		newSession.UserID, string(newSession.ClientType), newSession.DeviceName, newSession.OAuthClientID, newSession.Now,
		nextIdleExpiry(newSession.Now, absoluteExpiresAt), absoluteExpiresAt).Scan(&sessionID)
	if err != nil {
		return Session{}, fmt.Errorf("create session: %w", err)
	}
	return scanSingleSession(db.QueryRow(ctx, `
		SELECT `+sessionColumns+`
		FROM sessions JOIN users ON users.id = sessions.user_id
		WHERE sessions.id = $1`, sessionID))
}

func (Repository) FindActiveByToken(ctx context.Context, db platform.DBTX, token string, now time.Time) (Session, error) {
	return scanSingleSession(db.QueryRow(ctx, `
		SELECT `+sessionColumns+`
		FROM sessions JOIN users ON users.id = sessions.user_id
		WHERE sessions.token = $1 AND sessions.revoked_at IS NULL
		  AND sessions.idle_expires_at > $2 AND sessions.absolute_expires_at > $2
		  AND users.deleted_at IS NULL AND users.deactivated_at IS NULL`, token, now))
}

func (Repository) ListActiveForUser(ctx context.Context, db platform.DBTX, userID int64, now time.Time) ([]Session, error) {
	rows, err := db.Query(ctx, `
		SELECT `+sessionColumns+`
		FROM sessions JOIN users ON users.id = sessions.user_id
		WHERE sessions.user_id = $1 AND sessions.revoked_at IS NULL
		  AND sessions.idle_expires_at > $2 AND sessions.absolute_expires_at > $2
		ORDER BY sessions.last_used_at DESC`, userID, now)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	sessions, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Session, error) {
		return scanSession(row)
	})
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	return sessions, nil
}

func (Repository) Revoke(ctx context.Context, db platform.DBTX, sessionID int64, now time.Time) error {
	_, err := db.Exec(ctx, `UPDATE sessions SET revoked_at = $2 WHERE id = $1 AND revoked_at IS NULL`, sessionID, now)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

func (Repository) RevokeAllForUser(ctx context.Context, db platform.DBTX, userID int64, now time.Time) error {
	_, err := db.Exec(ctx, `UPDATE sessions SET revoked_at = $2 WHERE user_id = $1 AND revoked_at IS NULL`, userID, now)
	if err != nil {
		return fmt.Errorf("revoke user sessions: %w", err)
	}
	return nil
}

func (Repository) RevokeForUser(ctx context.Context, db platform.DBTX, userID int64, sessionToken string, now time.Time) error {
	tag, err := db.Exec(ctx, `
		UPDATE sessions SET revoked_at = $3
		WHERE user_id = $1 AND token = $2 AND revoked_at IS NULL`, userID, sessionToken, now)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSessionNotFound
	}
	return nil
}

func (Repository) ExtendIdleExpiry(ctx context.Context, db platform.DBTX, session Session, now time.Time) (Session, error) {
	session.LastUsedAt = now
	session.IdleExpiresAt = nextIdleExpiry(now, session.AbsoluteExpiresAt)
	_, err := db.Exec(ctx, `UPDATE sessions SET last_used_at = $2, idle_expires_at = $3 WHERE id = $1`,
		session.ID, session.LastUsedAt, session.IdleExpiresAt)
	if err != nil {
		return Session{}, fmt.Errorf("extend session: %w", err)
	}
	return session, nil
}

func (Repository) CreateRefreshToken(ctx context.Context, db platform.DBTX, sessionID int64, now time.Time) (string, error) {
	secret := platform.NewBearerSecret()
	_, err := db.Exec(ctx, `INSERT INTO refresh_tokens (session_id, token_hash, created_at) VALUES ($1, $2, $3)`,
		sessionID, platform.HashBearerSecret(secret), now)
	if err != nil {
		return "", fmt.Errorf("create refresh token: %w", err)
	}
	return secret, nil
}

type lockedRefreshToken struct {
	ID      int64
	UsedAt  *time.Time
	Session Session
}

func (Repository) LockRefreshToken(ctx context.Context, db platform.DBTX, secret string) (lockedRefreshToken, error) {
	var locked lockedRefreshToken
	var session Session
	var clientType string
	err := db.QueryRow(ctx, `
		SELECT refresh_tokens.id, refresh_tokens.used_at, `+sessionColumns+`
		FROM refresh_tokens
		JOIN sessions ON sessions.id = refresh_tokens.session_id
		JOIN users ON users.id = sessions.user_id
		WHERE refresh_tokens.token_hash = $1
		FOR UPDATE OF refresh_tokens, sessions`, platform.HashBearerSecret(secret)).Scan(
		&locked.ID, &locked.UsedAt,
		&session.ID, &session.Token, &session.UserID, &session.UserToken, &session.RoleID, &clientType, &session.DeviceName,
		&session.OAuthClientID, &session.CreatedAt, &session.LastUsedAt, &session.IdleExpiresAt, &session.AbsoluteExpiresAt,
		&session.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return lockedRefreshToken{}, ErrRefreshTokenNotFound
	}
	if err != nil {
		return lockedRefreshToken{}, fmt.Errorf("find refresh token: %w", err)
	}
	session.ClientType = ClientType(clientType)
	locked.Session = session
	return locked, nil
}

func (Repository) MarkRefreshTokenUsed(ctx context.Context, db platform.DBTX, refreshTokenID int64, now time.Time) error {
	_, err := db.Exec(ctx, `UPDATE refresh_tokens SET used_at = $2 WHERE id = $1`, refreshTokenID, now)
	if err != nil {
		return fmt.Errorf("mark refresh token used: %w", err)
	}
	return nil
}

func scanSingleSession(row pgx.Row) (Session, error) {
	session, err := scanSession(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrSessionNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("find session: %w", err)
	}
	return session, nil
}

func scanSession(row pgx.Row) (Session, error) {
	var session Session
	var clientType string
	err := row.Scan(&session.ID, &session.Token, &session.UserID, &session.UserToken, &session.RoleID, &clientType,
		&session.DeviceName, &session.OAuthClientID, &session.CreatedAt, &session.LastUsedAt, &session.IdleExpiresAt,
		&session.AbsoluteExpiresAt, &session.RevokedAt)
	session.ClientType = ClientType(clientType)
	return session, err
}
