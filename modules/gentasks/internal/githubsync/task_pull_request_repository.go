package githubsync

import (
	"context"
	"fmt"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
	"github.com/jackc/pgx/v5"
)

type TaskPullRequestRepository struct{}

const taskPullRequestColumns = `id, token, task_id, repository, pull_request_number, last_moved_event, moved_at, merge_moved_at,
	created_at, updated_at`

// RecordMove keeps one row per task and pull request; once a merge set merge_moved_at, later moves never clear it.
func (TaskPullRequestRepository) RecordMove(ctx context.Context, db platform.DBTX, move TaskPullRequestMove) error {
	_, err := db.Exec(ctx, `
		INSERT INTO task_pull_requests (token, task_id, repository, pull_request_number, last_moved_event, moved_at, merge_moved_at,
		                                created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, CASE WHEN $5 = $7 THEN $6::timestamptz END, $6, $6)
		ON CONFLICT (task_id, repository, pull_request_number) DO UPDATE SET
			last_moved_event = EXCLUDED.last_moved_event,
			moved_at = EXCLUDED.moved_at,
			merge_moved_at = coalesce(task_pull_requests.merge_moved_at, EXCLUDED.merge_moved_at),
			updated_at = EXCLUDED.updated_at`,
		platform.NewPublicToken(), move.TaskID, move.Repository, move.PullRequestNumber, move.Trigger, move.At, TriggerMerged)
	if err != nil {
		return fmt.Errorf("record task pull request move: %w", err)
	}
	return nil
}

func (TaskPullRequestRepository) ListByTask(ctx context.Context, db platform.DBTX, taskID int64) ([]TaskPullRequest, error) {
	rows, err := db.Query(ctx, `
		SELECT `+taskPullRequestColumns+` FROM task_pull_requests
		WHERE task_id = $1
		ORDER BY repository, pull_request_number`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list task pull requests: %w", err)
	}
	links, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (TaskPullRequest, error) {
		var link TaskPullRequest
		err := row.Scan(&link.ID, &link.Token, &link.TaskID, &link.Repository, &link.PullRequestNumber, &link.LastMovedEvent,
			&link.MovedAt, &link.MergeMovedAt, &link.CreatedAt, &link.UpdatedAt)
		return link, err
	})
	if err != nil {
		return nil, fmt.Errorf("list task pull requests: %w", err)
	}
	return links, nil
}

// ListMergeLockedTaskIDs returns which of the given tasks a merge already moved, in one query (rule 21).
func (TaskPullRequestRepository) ListMergeLockedTaskIDs(ctx context.Context, db platform.DBTX, taskIDs []int64) (map[int64]bool, error) {
	rows, err := db.Query(ctx, `
		SELECT DISTINCT task_id FROM task_pull_requests
		WHERE task_id = ANY($1) AND merge_moved_at IS NOT NULL`, taskIDs)
	if err != nil {
		return nil, fmt.Errorf("list merge-locked tasks: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return nil, fmt.Errorf("list merge-locked tasks: %w", err)
	}
	locked := make(map[int64]bool, len(ids))
	for _, id := range ids {
		locked[id] = true
	}
	return locked, nil
}
