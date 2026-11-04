package githubsync

import (
	"context"
	"fmt"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
	"github.com/jackc/pgx/v5"
)

type TaskPullRequestRepository struct{}

const taskPullRequestColumns = `task_pull_requests.id, task_pull_requests.token, task_pull_requests.task_id,
	task_pull_requests.pull_request_id, task_pull_requests.last_moved_event, task_pull_requests.moved_at,
	task_pull_requests.merge_moved_at, task_pull_requests.created_at, task_pull_requests.updated_at, task_pull_requests.deleted_at`

// RecordMove keeps one active link per task and pull request; once a merge set merge_moved_at, later moves never clear it.
func (TaskPullRequestRepository) RecordMove(ctx context.Context, db platform.DBTX, move TaskPullRequestMove) error {
	_, err := db.Exec(ctx, `
		INSERT INTO task_pull_requests (token, task_id, pull_request_id, last_moved_event, moved_at, merge_moved_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, CASE WHEN $4 = $6 THEN $5::timestamptz END, $5, $5)
		ON CONFLICT (task_id, pull_request_id) WHERE deleted_at IS NULL DO UPDATE SET
			last_moved_event = EXCLUDED.last_moved_event,
			moved_at = EXCLUDED.moved_at,
			merge_moved_at = coalesce(task_pull_requests.merge_moved_at, EXCLUDED.merge_moved_at),
			updated_at = EXCLUDED.updated_at`,
		platform.NewPublicToken(), move.TaskID, move.PullRequestID, move.Trigger, move.At, TriggerMerged)
	if err != nil {
		return fmt.Errorf("record task pull request move: %w", err)
	}
	return nil
}

// Link adds an active link from each task to the pull request, in one statement (rule 21); existing links are kept.
func (TaskPullRequestRepository) Link(ctx context.Context, db platform.DBTX, pullRequestID int64, taskIDs []int64, at time.Time) error {
	if len(taskIDs) == 0 {
		return nil
	}
	tokens := make([]string, 0, len(taskIDs))
	for range taskIDs {
		tokens = append(tokens, platform.NewPublicToken())
	}
	_, err := db.Exec(ctx, `
		INSERT INTO task_pull_requests (token, task_id, pull_request_id, created_at, updated_at)
		SELECT linked.token, linked.task_id, $3, $4, $4
		FROM unnest($1::text[], $2::bigint[]) AS linked (token, task_id)
		ON CONFLICT (task_id, pull_request_id) WHERE deleted_at IS NULL DO NOTHING`, tokens, taskIDs, pullRequestID, at)
	if err != nil {
		return fmt.Errorf("link tasks to pull request: %w", err)
	}
	return nil
}

// UnlinkUncited removes the pull request's links to every task whose key is not among cited. It matches keys rather
// than the cited active tasks, so a pull request still citing an archived task stays linked to it.
func (TaskPullRequestRepository) UnlinkUncited(ctx context.Context, db platform.DBTX, pullRequestID int64, cited []TaskKey, at time.Time) error {
	prefixes, numbers := taskKeyColumns(cited)
	_, err := db.Exec(ctx, `
		UPDATE task_pull_requests SET deleted_at = $4, updated_at = $4
		FROM tasks
		JOIN boards ON boards.id = tasks.board_id
		WHERE task_pull_requests.task_id = tasks.id
		  AND task_pull_requests.pull_request_id = $1
		  AND task_pull_requests.deleted_at IS NULL
		  AND NOT EXISTS (
		      SELECT 1 FROM unnest($2::text[], $3::integer[]) AS cited (prefix, number)
		      WHERE cited.prefix = boards.task_prefix AND cited.number = tasks.number
		  )`, pullRequestID, prefixes, numbers, at)
	if err != nil {
		return fmt.Errorf("unlink uncited tasks: %w", err)
	}
	return nil
}

// ListActiveForTasks returns each task's active links with their pull requests, in one query (rule 21).
func (TaskPullRequestRepository) ListActiveForTasks(ctx context.Context, db platform.DBTX, taskIDs []int64) (map[int64][]LinkedPullRequest, error) {
	rows, err := db.Query(ctx, `
		SELECT `+githubPullRequestColumns+`, `+taskPullRequestColumns+`
		FROM task_pull_requests
		JOIN github_pull_requests ON github_pull_requests.id = task_pull_requests.pull_request_id
		WHERE task_pull_requests.task_id = ANY($1) AND task_pull_requests.deleted_at IS NULL
		ORDER BY task_pull_requests.task_id, github_pull_requests.repository, github_pull_requests.number`, taskIDs)
	if err != nil {
		return nil, fmt.Errorf("list task pull requests: %w", err)
	}
	linked, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (LinkedPullRequest, error) {
		var link TaskPullRequest
		pullRequest, err := scanGithubPullRequest(row, &link.ID, &link.Token, &link.TaskID, &link.PullRequestID, &link.LastMovedEvent,
			&link.MovedAt, &link.MergeMovedAt, &link.CreatedAt, &link.UpdatedAt, &link.DeletedAt)
		return LinkedPullRequest{Link: link, PullRequest: pullRequest}, err
	})
	if err != nil {
		return nil, fmt.Errorf("list task pull requests: %w", err)
	}
	byTask := make(map[int64][]LinkedPullRequest, len(taskIDs))
	for _, pullRequest := range linked {
		byTask[pullRequest.Link.TaskID] = append(byTask[pullRequest.Link.TaskID], pullRequest)
	}
	return byTask, nil
}

// ListMergeLockedTaskIDs returns which of the given tasks a merge already moved, in one query (rule 21). Removed links
// count too: unlinking a pull request never releases the lock its merge set.
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
