package runs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform"
	"github.com/jackc/pgx/v5"
)

type EntryRepository struct{}

type NewEntry struct {
	ConversationID int64
	RunID          *int64
	RunSequence    *int64
	Type           EntryType
	AuthorToken    *string
	Payload        any
	At             time.Time
}

const entrySelect = `
SELECT conversation_entries.id, conversation_entries.token, conversation_entries.sequence, runs.token, conversation_entries.run_sequence,
       conversation_entries.type, conversation_entries.author_token, conversation_entries.payload, conversation_entries.delivered_at,
       conversation_entries.created_at
FROM conversation_entries
LEFT JOIN runs ON runs.id = conversation_entries.run_id`

// Append adds the entry with the next sequence of its conversation. It locks the conversation row, so two writers never
// take the same sequence. A run event whose run sequence was already stored is ignored and reported as not added.
func (EntryRepository) Append(ctx context.Context, db platform.DBTX, newEntry NewEntry) (bool, error) {
	if _, err := db.Exec(ctx, `SELECT 1 FROM conversations WHERE id = $1 FOR UPDATE`, newEntry.ConversationID); err != nil {
		return false, fmt.Errorf("lock conversation: %w", err)
	}
	payload, err := json.Marshal(newEntry.Payload)
	if err != nil {
		return false, fmt.Errorf("encode entry payload: %w", err)
	}
	tag, err := db.Exec(ctx, `
		INSERT INTO conversation_entries (token, conversation_id, run_id, sequence, run_sequence, type, author_token, payload, created_at)
		VALUES ($1, $2, $3, (SELECT COALESCE(MAX(sequence), 0) + 1 FROM conversation_entries WHERE conversation_id = $2), $4, $5, $6, $7, $8)
		ON CONFLICT (run_id, run_sequence) WHERE run_sequence IS NOT NULL DO NOTHING`,
		platform.NewPublicToken(), newEntry.ConversationID, newEntry.RunID, newEntry.RunSequence, newEntry.Type, newEntry.AuthorToken, payload, newEntry.At)
	if err != nil {
		return false, fmt.Errorf("append entry: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// ListAfter returns the entries after a sequence, so clients poll cheaply.
func (EntryRepository) ListAfter(ctx context.Context, db platform.DBTX, conversationID, afterSequence int64) ([]Entry, error) {
	rows, err := db.Query(ctx, entrySelect+` WHERE conversation_entries.conversation_id = $1 AND conversation_entries.sequence > $2
		ORDER BY conversation_entries.sequence`, conversationID, afterSequence)
	if err != nil {
		return nil, fmt.Errorf("list entries: %w", err)
	}
	listed, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Entry, error) { return scanEntry(row) })
	if err != nil {
		return nil, fmt.Errorf("list entries: %w", err)
	}
	return listed, nil
}

// ListUndeliveredMessages are the human messages a runner has not acknowledged yet, in order.
func (EntryRepository) ListUndeliveredMessages(ctx context.Context, db platform.DBTX, conversationID int64) ([]Entry, error) {
	rows, err := db.Query(ctx, entrySelect+` WHERE conversation_entries.conversation_id = $1 AND conversation_entries.type = 'human_message'
		AND conversation_entries.delivered_at IS NULL ORDER BY conversation_entries.sequence`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("list undelivered messages: %w", err)
	}
	listed, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Entry, error) { return scanEntry(row) })
	if err != nil {
		return nil, fmt.Errorf("list undelivered messages: %w", err)
	}
	return listed, nil
}

func (EntryRepository) MarkDelivered(ctx context.Context, db platform.DBTX, conversationID, upToSequence int64, at time.Time) error {
	_, err := db.Exec(ctx, `
		UPDATE conversation_entries SET delivered_at = $3
		WHERE conversation_id = $1 AND type = 'human_message' AND delivered_at IS NULL AND sequence <= $2`, conversationID, upToSequence, at)
	if err != nil {
		return fmt.Errorf("mark messages delivered: %w", err)
	}
	return nil
}

func (EntryRepository) LastSequence(ctx context.Context, db platform.DBTX, conversationID int64) (int64, error) {
	var sequence int64
	err := db.QueryRow(ctx, `SELECT COALESCE(MAX(sequence), 0) FROM conversation_entries WHERE conversation_id = $1`, conversationID).Scan(&sequence)
	if err != nil {
		return 0, fmt.Errorf("find last sequence: %w", err)
	}
	return sequence, nil
}

func scanEntry(row pgx.Row) (Entry, error) {
	var entry Entry
	err := row.Scan(&entry.ID, &entry.Token, &entry.Sequence, &entry.RunToken, &entry.RunSequence, &entry.Type, &entry.AuthorToken,
		&entry.Payload, &entry.DeliveredAt, &entry.CreatedAt)
	return entry, err
}

// DeleteByConversation removes every entry of a conversation, for a person clearing it (a documented rule 22 exception).
func (EntryRepository) DeleteByConversation(ctx context.Context, db platform.DBTX, conversationID int64) error {
	if _, err := db.Exec(ctx, `DELETE FROM conversation_entries WHERE conversation_id = $1`, conversationID); err != nil {
		return fmt.Errorf("delete conversation entries: %w", err)
	}
	return nil
}
