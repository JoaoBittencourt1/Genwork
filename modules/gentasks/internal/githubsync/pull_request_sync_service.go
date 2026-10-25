package githubsync

import (
	"context"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TaskMover is the tasks feature's move, run inside the caller's transaction while it holds the board locks.
type TaskMover interface {
	MoveTaskToColumnEnd(ctx context.Context, tx pgx.Tx, taskID, columnID int64) error
}

// PullRequestSyncService keeps the snapshot of a pull request, links it to the tasks it cites, and moves them. It is
// idempotent — snapshots only move forward in GitHub's time, moves only go forward, and a merge locks the task — so a
// redelivered or replayed event changes nothing (design §7 of gentasks-github-sync).
type PullRequestSyncService struct {
	db           *pgxpool.Pool
	citedTasks   CitedTaskRepository
	settings     BoardGithubSettingsRepository
	pullRequests GithubPullRequestRepository
	links        TaskPullRequestRepository
	mover        TaskMover
	now          func() time.Time
}

func NewPullRequestSyncService(db *pgxpool.Pool, mover TaskMover, now func() time.Time) *PullRequestSyncService {
	return &PullRequestSyncService{db: db, mover: mover, now: now}
}

func (service *PullRequestSyncService) SyncPullRequest(ctx context.Context, event PullRequestEvent) error {
	if !event.Action.IsHandled() {
		return nil
	}
	keys := ExtractTaskKeys(event.Title, event.HeadRef)
	return pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		cited, err := service.lockCitedTasks(ctx, tx, keys)
		if err != nil {
			return err
		}
		pullRequestID, known, err := service.recordSnapshot(ctx, tx, event, keys, cited)
		if err != nil || !known {
			return err
		}
		return service.moveCitedTasks(ctx, tx, event, cited, pullRequestID)
	})
}

// lockCitedTasks locks the cited tasks' boards, then reads the tasks again so their columns are current under the lock.
func (service *PullRequestSyncService) lockCitedTasks(ctx context.Context, tx pgx.Tx, keys []TaskKey) ([]CitedTask, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	cited, err := service.citedTasks.ListByKeys(ctx, tx, keys)
	if err != nil || len(cited) == 0 {
		return nil, err
	}
	if err := platform.LockBoards(ctx, tx, boardIDsOf(cited)); err != nil {
		return nil, err
	}
	return service.citedTasks.ListByKeys(ctx, tx, keys)
}

// recordSnapshot stores what the delivery says about the pull request and relinks it to the tasks it cites now. A pull
// request citing no task is stored only when gentasks already knows it, which is how its last tasks get unlinked;
// known is false for any other. A delivery older than the stored snapshot changes neither the snapshot nor the links.
func (service *PullRequestSyncService) recordSnapshot(ctx context.Context, tx pgx.Tx, event PullRequestEvent, keys []TaskKey, cited []CitedTask) (pullRequestID int64, known bool, err error) {
	at := service.now()
	var saved SavedPullRequest
	if len(cited) == 0 {
		saved, known, err = service.pullRequests.UpdateKnown(ctx, tx, event.Snapshot(), at)
		if err != nil || !known {
			return 0, false, err
		}
	} else if saved, err = service.pullRequests.Upsert(ctx, tx, event.Snapshot(), at); err != nil {
		return 0, false, err
	}
	if !saved.Current {
		return saved.ID, true, nil
	}
	if err := service.links.UnlinkUncited(ctx, tx, saved.ID, keys, at); err != nil {
		return 0, false, err
	}
	return saved.ID, true, service.links.Link(ctx, tx, saved.ID, taskIDsOf(cited), at)
}

// moveCitedTasks applies the movement rules (design §9 of gentasks-github-sync) to every cited task. It also runs for a
// delivery older than the stored snapshot: moves only go forward, so a late delivery can never undo a newer one.
func (service *PullRequestSyncService) moveCitedTasks(ctx context.Context, tx pgx.Tx, event PullRequestEvent, cited []CitedTask, pullRequestID int64) error {
	trigger, moves := event.Trigger()
	if !moves || len(cited) == 0 {
		return nil
	}
	settingsByBoard, err := service.settings.ListByBoards(ctx, tx, boardIDsOf(cited))
	if err != nil {
		return err
	}
	locked, err := service.links.ListMergeLockedTaskIDs(ctx, tx, taskIDsOf(cited))
	if err != nil {
		return err
	}
	for _, task := range cited {
		target := event.TargetColumn(task, trigger, settingsByBoard[task.BoardID], locked[task.TaskID])
		if target == nil {
			continue
		}
		if err := service.mover.MoveTaskToColumnEnd(ctx, tx, task.TaskID, target.ID); err != nil {
			return err
		}
		move := TaskPullRequestMove{TaskID: task.TaskID, PullRequestID: pullRequestID, Trigger: trigger, At: service.now()}
		if err := service.links.RecordMove(ctx, tx, move); err != nil {
			return err
		}
	}
	return nil
}

func boardIDsOf(cited []CitedTask) []int64 {
	ids := make([]int64, 0, len(cited))
	for _, task := range cited {
		ids = append(ids, task.BoardID)
	}
	return ids
}

func taskIDsOf(cited []CitedTask) []int64 {
	ids := make([]int64, 0, len(cited))
	for _, task := range cited {
		ids = append(ids, task.TaskID)
	}
	return ids
}
