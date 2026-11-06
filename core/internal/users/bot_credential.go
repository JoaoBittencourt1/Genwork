package users

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/jackc/pgx/v5"
)

var ErrBotCredentialNotFound = errors.New("bot credential not found")

type BotCredential struct {
	ID         int64
	Token      string
	UserID     int64
	SecretHash []byte
	CreatedAt  time.Time
	RotatedAt  *time.Time
}

func HashBotSecret(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

func (credential BotCredential) Matches(secret string) bool {
	return subtle.ConstantTimeCompare(credential.SecretHash, HashBotSecret(secret)) == 1
}

type BotCredentialRepository struct{}

const botCredentialColumns = `id, token, user_id, secret_hash, created_at, rotated_at`

func (BotCredentialRepository) Create(ctx context.Context, db platform.DBTX, userID int64, secretHash []byte) (BotCredential, error) {
	credential, err := scanBotCredential(db.QueryRow(ctx, `
		INSERT INTO bot_credentials (user_id, secret_hash) VALUES ($1, $2)
		RETURNING `+botCredentialColumns, userID, secretHash))
	if err != nil {
		return BotCredential{}, fmt.Errorf("create bot credential: %w", err)
	}
	return credential, nil
}

// FindActiveByBotToken finds the credential of a bot member that is neither removed nor deactivated.
func (BotCredentialRepository) FindActiveByBotToken(ctx context.Context, db platform.DBTX, botToken string) (BotCredential, error) {
	credential, err := scanBotCredential(db.QueryRow(ctx, `
		SELECT bot_credentials.id, bot_credentials.token, bot_credentials.user_id, bot_credentials.secret_hash,
			bot_credentials.created_at, bot_credentials.rotated_at
		FROM bot_credentials JOIN users ON users.id = bot_credentials.user_id
		WHERE users.token = $1 AND users.kind = 'bot' AND users.deleted_at IS NULL AND users.deactivated_at IS NULL`, botToken))
	if errors.Is(err, pgx.ErrNoRows) {
		return BotCredential{}, ErrBotCredentialNotFound
	}
	if err != nil {
		return BotCredential{}, fmt.Errorf("find bot credential: %w", err)
	}
	return credential, nil
}

func (BotCredentialRepository) Rotate(ctx context.Context, db platform.DBTX, userID int64, secretHash []byte, at time.Time) error {
	tag, err := db.Exec(ctx, `UPDATE bot_credentials SET secret_hash = $2, rotated_at = $3 WHERE user_id = $1`, userID, secretHash, at)
	if err != nil {
		return fmt.Errorf("rotate bot credential: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrBotCredentialNotFound
	}
	return nil
}

func scanBotCredential(row pgx.Row) (BotCredential, error) {
	var credential BotCredential
	err := row.Scan(&credential.ID, &credential.Token, &credential.UserID, &credential.SecretHash, &credential.CreatedAt, &credential.RotatedAt)
	return credential, err
}
