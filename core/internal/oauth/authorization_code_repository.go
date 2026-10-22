package oauth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/jackc/pgx/v5"
)

var errCodeNotFound = errors.New("authorization code not found")

type AuthorizationCodeRepository struct{}

type NewAuthorizationCode struct {
	ClientID      int64
	UserID        int64
	RedirectURI   string
	CodeChallenge string
	Scope         string
	Now           time.Time
}

// Create returns the code itself, which leaves core only in the redirect to the client.
func (AuthorizationCodeRepository) Create(ctx context.Context, db platform.DBTX, newCode NewAuthorizationCode) (string, error) {
	code := platform.NewBearerSecret()
	_, err := db.Exec(ctx, `
		INSERT INTO oauth_authorization_codes (code_hash, client_id, user_id, redirect_uri, code_challenge, scope, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		platform.HashBearerSecret(code), newCode.ClientID, newCode.UserID, newCode.RedirectURI, newCode.CodeChallenge, newCode.Scope,
		newCode.Now.Add(AuthorizationCodeLifetime), newCode.Now)
	if err != nil {
		return "", fmt.Errorf("create authorization code: %w", err)
	}
	return code, nil
}

// LockByCode holds the row until the exchange commits, so two exchanges of one code cannot both succeed.
func (AuthorizationCodeRepository) LockByCode(ctx context.Context, db platform.DBTX, code string) (AuthorizationCode, error) {
	var found AuthorizationCode
	err := db.QueryRow(ctx, `
		SELECT id, client_id, user_id, redirect_uri, code_challenge, scope, expires_at, used_at, session_id
		FROM oauth_authorization_codes WHERE code_hash = $1
		FOR UPDATE`, platform.HashBearerSecret(code)).Scan(
		&found.ID, &found.ClientID, &found.UserID, &found.RedirectURI, &found.CodeChallenge, &found.Scope,
		&found.ExpiresAt, &found.UsedAt, &found.SessionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return AuthorizationCode{}, errCodeNotFound
	}
	if err != nil {
		return AuthorizationCode{}, fmt.Errorf("find authorization code: %w", err)
	}
	return found, nil
}

func (AuthorizationCodeRepository) MarkUsed(ctx context.Context, db platform.DBTX, codeID int64, sessionID int64, now time.Time) error {
	_, err := db.Exec(ctx, `UPDATE oauth_authorization_codes SET used_at = $2, session_id = $3 WHERE id = $1`, codeID, now, sessionID)
	if err != nil {
		return fmt.Errorf("mark authorization code used: %w", err)
	}
	return nil
}
