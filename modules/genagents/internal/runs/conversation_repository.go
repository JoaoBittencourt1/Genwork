package runs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform"
	"github.com/jackc/pgx/v5"
)

type ConversationRepository struct{}

const conversationColumns = `id, token, kind, task_token, owner_user_token, title, bot_id, delegated_session, delegated_refresh_ciphertext, created_at, updated_at`

// FindOrCreateTask returns the one conversation of a task, creating it on first use.
func (repository ConversationRepository) FindOrCreateTask(ctx context.Context, db platform.DBTX, taskToken string, at time.Time) (Conversation, error) {
	_, err := db.Exec(ctx, `
		INSERT INTO conversations (token, kind, task_token, created_at, updated_at) VALUES ($1, 'task', $2, $3, $3)
		ON CONFLICT (task_token) WHERE deleted_at IS NULL AND kind = 'task' DO NOTHING`,
		platform.NewPublicToken(), taskToken, at)
	if err != nil {
		return Conversation{}, fmt.Errorf("create task conversation: %w", err)
	}
	return repository.FindByTaskToken(ctx, db, taskToken)
}

func (repository ConversationRepository) FindByTaskToken(ctx context.Context, db platform.DBTX, taskToken string) (Conversation, error) {
	return repository.findOne(ctx, db, `task_token = $1 AND kind = 'task'`, taskToken)
}

func (repository ConversationRepository) FindByToken(ctx context.Context, db platform.DBTX, token string) (Conversation, error) {
	return repository.findOne(ctx, db, `token = $1`, token)
}

func (repository ConversationRepository) FindByID(ctx context.Context, db platform.DBTX, id int64) (Conversation, error) {
	return repository.findOne(ctx, db, `id = $1`, id)
}

func (ConversationRepository) findOne(ctx context.Context, db platform.DBTX, condition string, argument any) (Conversation, error) {
	conversation, err := scanConversation(db.QueryRow(ctx, `SELECT `+conversationColumns+` FROM conversations WHERE `+condition+` AND deleted_at IS NULL`, argument))
	if errors.Is(err, pgx.ErrNoRows) {
		return Conversation{}, ErrConversationNotFound
	}
	if err != nil {
		return Conversation{}, fmt.Errorf("find conversation: %w", err)
	}
	return conversation, nil
}

func (repository ConversationRepository) CreateAdminChat(ctx context.Context, db platform.DBTX, ownerUserToken, title string, botID int64, delegatedSession string, delegatedRefresh []byte, at time.Time) (Conversation, error) {
	token := platform.NewPublicToken()
	_, err := db.Exec(ctx, `
		INSERT INTO conversations (token, kind, owner_user_token, title, bot_id, delegated_session, delegated_refresh_ciphertext, created_at, updated_at)
		VALUES ($1, 'admin_chat', $2, $3, $4, $5, $6, $7, $7)`, token, ownerUserToken, title, botID, delegatedSession, delegatedRefresh, at)
	if err != nil {
		return Conversation{}, fmt.Errorf("create admin chat: %w", err)
	}
	return repository.FindByToken(ctx, db, token)
}

func (ConversationRepository) ListAdminChats(ctx context.Context, db platform.DBTX, ownerUserToken string) ([]Conversation, error) {
	rows, err := db.Query(ctx, `
		SELECT `+conversationColumns+` FROM conversations
		WHERE kind = 'admin_chat' AND owner_user_token = $1 AND deleted_at IS NULL ORDER BY updated_at DESC, id DESC`, ownerUserToken)
	if err != nil {
		return nil, fmt.Errorf("list admin chats: %w", err)
	}
	listed, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Conversation, error) { return scanConversation(row) })
	if err != nil {
		return nil, fmt.Errorf("list admin chats: %w", err)
	}
	return listed, nil
}

func (ConversationRepository) UpdateTitle(ctx context.Context, db platform.DBTX, id int64, title string, at time.Time) error {
	_, err := db.Exec(ctx, `UPDATE conversations SET title = $2, updated_at = $3 WHERE id = $1 AND deleted_at IS NULL`, id, title, at)
	if err != nil {
		return fmt.Errorf("rename conversation: %w", err)
	}
	return nil
}

func (ConversationRepository) UpdateDelegatedSession(ctx context.Context, db platform.DBTX, id int64, session string, refreshCiphertext []byte, at time.Time) error {
	_, err := db.Exec(ctx, `
		UPDATE conversations SET delegated_session = $2, delegated_refresh_ciphertext = $3, updated_at = $4 WHERE id = $1 AND deleted_at IS NULL`,
		id, session, refreshCiphertext, at)
	if err != nil {
		return fmt.Errorf("update delegated session: %w", err)
	}
	return nil
}

func (ConversationRepository) Touch(ctx context.Context, db platform.DBTX, id int64, at time.Time) error {
	_, err := db.Exec(ctx, `UPDATE conversations SET updated_at = $2 WHERE id = $1`, id, at)
	if err != nil {
		return fmt.Errorf("touch conversation: %w", err)
	}
	return nil
}

// SoftDelete clears the delegated refresh token in the same statement (rules 22 and 67).
func (ConversationRepository) SoftDelete(ctx context.Context, db platform.DBTX, id int64, at time.Time) error {
	_, err := db.Exec(ctx, `
		UPDATE conversations SET deleted_at = $2, updated_at = $2, delegated_refresh_ciphertext = NULL WHERE id = $1 AND deleted_at IS NULL`, id, at)
	if err != nil {
		return fmt.Errorf("delete conversation: %w", err)
	}
	return nil
}

func scanConversation(row pgx.Row) (Conversation, error) {
	var conversation Conversation
	err := row.Scan(&conversation.ID, &conversation.Token, &conversation.Kind, &conversation.TaskToken, &conversation.OwnerUserToken,
		&conversation.Title, &conversation.BotID, &conversation.DelegatedSession, &conversation.DelegatedRefresh,
		&conversation.CreatedAt, &conversation.UpdatedAt)
	return conversation, err
}
