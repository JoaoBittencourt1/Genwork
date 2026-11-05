package runs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform"
	"github.com/jackc/pgx/v5"
)

type RunRepository struct{}

type NewRun struct {
	ConversationID     int64
	BotID              int64
	Kind               RunKind
	RequesterUserToken string
	CLISessionID       *string
	Spec               RunSpec
	At                 time.Time
}

const runSelect = `
SELECT runs.id, runs.token, runs.conversation_id, conversations.token, runs.bot_id, runs.kind, runs.state, runs.failure_reason,
       runs.requester_user_token, runs.runner_id, runs.cli_session_id, runs.spec, runs.waiting_since, runs.timeout_at, runs.created_at,
       runs.claimed_at, runs.ended_at
FROM runs
JOIN conversations ON conversations.id = runs.conversation_id`

func (repository RunRepository) Create(ctx context.Context, db platform.DBTX, newRun NewRun) (Run, error) {
	spec, err := json.Marshal(newRun.Spec)
	if err != nil {
		return Run{}, fmt.Errorf("encode run spec: %w", err)
	}
	token := platform.NewPublicToken()
	_, err = db.Exec(ctx, `
		INSERT INTO runs (token, conversation_id, bot_id, kind, state, requester_user_token, cli_session_id, spec, created_at)
		VALUES ($1, $2, $3, $4, 'queued', $5, $6, $7, $8)`,
		token, newRun.ConversationID, newRun.BotID, newRun.Kind, newRun.RequesterUserToken, newRun.CLISessionID, spec, newRun.At)
	if platform.IsUniqueViolation(err) {
		return Run{}, ErrRunAlreadyActive
	}
	if err != nil {
		return Run{}, fmt.Errorf("create run: %w", err)
	}
	return repository.FindByToken(ctx, db, token)
}

func (repository RunRepository) FindByToken(ctx context.Context, db platform.DBTX, token string) (Run, error) {
	return repository.findOne(ctx, db, runSelect+` WHERE runs.token = $1`, token)
}

func (repository RunRepository) FindByID(ctx context.Context, db platform.DBTX, id int64) (Run, error) {
	return repository.findOne(ctx, db, runSelect+` WHERE runs.id = $1`, id)
}

// FindActive returns the active run of a conversation and kind, or ErrRunNotFound.
func (repository RunRepository) FindActive(ctx context.Context, db platform.DBTX, conversationID int64, kind RunKind) (Run, error) {
	return repository.findOne(ctx, db, runSelect+` WHERE runs.conversation_id = $1 AND runs.kind = $2
		AND runs.state IN ('queued', 'running', 'waiting_for_input')`, conversationID, kind)
}

// FindLast returns the most recent run of a conversation and kind, active or not.
func (repository RunRepository) FindLast(ctx context.Context, db platform.DBTX, conversationID int64, kind RunKind) (Run, error) {
	return repository.findOne(ctx, db, runSelect+` WHERE runs.conversation_id = $1 AND runs.kind = $2 ORDER BY runs.id DESC LIMIT 1`, conversationID, kind)
}

func (RunRepository) findOne(ctx context.Context, db platform.DBTX, query string, arguments ...any) (Run, error) {
	run, err := scanRun(db.QueryRow(ctx, query, arguments...))
	if errors.Is(err, pgx.ErrNoRows) {
		return Run{}, ErrRunNotFound
	}
	if err != nil {
		return Run{}, fmt.Errorf("find run: %w", err)
	}
	return run, nil
}

func (RunRepository) list(ctx context.Context, db platform.DBTX, query string, arguments ...any) ([]Run, error) {
	rows, err := db.Query(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	listed, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Run, error) { return scanRun(row) })
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	return listed, nil
}

// LockQueued locks the oldest queued runs without waiting for rows another claim holds, so two runners never take the
// same run (SELECT … FOR UPDATE SKIP LOCKED).
func (repository RunRepository) LockQueued(ctx context.Context, db platform.DBTX, limit int) ([]Run, error) {
	return repository.list(ctx, db, runSelect+` WHERE runs.state = 'queued' ORDER BY runs.created_at, runs.id LIMIT $1 FOR UPDATE OF runs SKIP LOCKED`, limit)
}

func (RunRepository) MarkClaimed(ctx context.Context, db platform.DBTX, id, runnerID int64, timeoutAt, at time.Time) error {
	_, err := db.Exec(ctx, `
		UPDATE runs SET state = 'running', runner_id = $2, claimed_at = $4, timeout_at = $3, waiting_since = NULL WHERE id = $1 AND state = 'queued'`,
		id, runnerID, timeoutAt, at)
	if err != nil {
		return fmt.Errorf("claim run: %w", err)
	}
	return nil
}

func (RunRepository) CountActiveForRunner(ctx context.Context, db platform.DBTX, runnerID int64) (int, error) {
	var count int
	err := db.QueryRow(ctx, `SELECT count(*) FROM runs WHERE runner_id = $1 AND state IN ('running', 'waiting_for_input')`, runnerID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count runner runs: %w", err)
	}
	return count, nil
}

func (repository RunRepository) ListActiveForRunner(ctx context.Context, db platform.DBTX, runnerID int64) ([]Run, error) {
	return repository.list(ctx, db, runSelect+` WHERE runs.runner_id = $1 AND runs.state IN ('running', 'waiting_for_input') ORDER BY runs.id`, runnerID)
}

func (repository RunRepository) ListActiveForBot(ctx context.Context, db platform.DBTX, botID int64) ([]Run, error) {
	return repository.list(ctx, db, runSelect+` WHERE runs.bot_id = $1 AND runs.state IN ('queued', 'running', 'waiting_for_input') ORDER BY runs.id`, botID)
}

func (repository RunRepository) ListActiveForConversation(ctx context.Context, db platform.DBTX, conversationID int64) ([]Run, error) {
	return repository.list(ctx, db, runSelect+` WHERE runs.conversation_id = $1 AND runs.state IN ('queued', 'running', 'waiting_for_input') ORDER BY runs.id`, conversationID)
}

// ListToSweep returns the active runs that a sweep may have to end: claimed ones for the runner check, and every one with
// a deadline.
func (repository RunRepository) ListToSweep(ctx context.Context, db platform.DBTX) ([]Run, error) {
	return repository.list(ctx, db, runSelect+` WHERE runs.state IN ('running', 'waiting_for_input') ORDER BY runs.id`)
}

// UpdateState moves a run, recording when it ended or began waiting; a failure carries its reason.
func (RunRepository) UpdateState(ctx context.Context, db platform.DBTX, id int64, state RunState, reason *FailureReason, at time.Time) error {
	var endedAt, waitingSince *time.Time
	switch {
	case !state.IsActive():
		endedAt = &at
	case state == RunStateWaitingForInput:
		waitingSince = &at
	}
	tag, err := db.Exec(ctx, `
		UPDATE runs SET state = $2, failure_reason = $3, ended_at = COALESCE($4, ended_at), waiting_since = $5
		WHERE id = $1 AND state IN ('queued', 'running', 'waiting_for_input')`, id, state, reason, endedAt, waitingSince)
	if err != nil {
		return fmt.Errorf("update run state: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrRunNotActive
	}
	return nil
}

func (RunRepository) UpdateSpec(ctx context.Context, db platform.DBTX, id int64, spec RunSpec) error {
	encoded, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("encode run spec: %w", err)
	}
	if _, err := db.Exec(ctx, `UPDATE runs SET spec = $2 WHERE id = $1`, id, encoded); err != nil {
		return fmt.Errorf("update run spec: %w", err)
	}
	return nil
}

func (RunRepository) SetCLISession(ctx context.Context, db platform.DBTX, id int64, sessionID string) error {
	_, err := db.Exec(ctx, `UPDATE runs SET cli_session_id = $2 WHERE id = $1`, id, sessionID)
	if err != nil {
		return fmt.Errorf("set cli session: %w", err)
	}
	return nil
}

func (RunRepository) ExtendTimeout(ctx context.Context, db platform.DBTX, id int64, timeoutAt time.Time) error {
	_, err := db.Exec(ctx, `UPDATE runs SET timeout_at = $2 WHERE id = $1`, id, timeoutAt)
	if err != nil {
		return fmt.Errorf("extend run timeout: %w", err)
	}
	return nil
}

func scanRun(row pgx.Row) (Run, error) {
	var run Run
	var spec []byte
	err := row.Scan(&run.ID, &run.Token, &run.ConversationID, &run.ConversationToken, &run.BotID, &run.Kind, &run.State, &run.FailureReason,
		&run.RequesterUserToken, &run.RunnerID, &run.CLISessionID, &spec, &run.WaitingSince, &run.TimeoutAt, &run.CreatedAt, &run.ClaimedAt, &run.EndedAt)
	if err != nil {
		return run, err
	}
	return run, json.Unmarshal(spec, &run.Spec)
}
