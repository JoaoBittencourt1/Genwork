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

// PullRequestSyncService moves the tasks a pull request cites. It is idempotent — moves only go forward and a merge
// locks the task — so a redelivered or replayed event changes nothing (design §7).
type PullRequestSyncService struct {
	db         *pgxpool.Pool
	citedTasks CitedTaskRepository
	settings   BoardGithubSettingsRepository
	links      TaskPullRequestRepository
	mover      TaskMover
	now        func() time.Time
}

func NewPullRequestSyncService(db *pgxpool.Pool, mover TaskMover, now func() time.Time) *PullRequestSyncService {
	return &PullRequestSyncService{db: db, mover: mover, now: now}
}

func (service *PullRequestSyncService) SyncPullRequest(ctx context.Context, event PullRequestEvent) error {
	trigger, moves := event.Trigger()
	keys := ExtractTaskKeys(event.Title, event.HeadRef)
	if !moves || len(keys) == 0 {
		return nil
	}
	return pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		cited, err := service.lockCitedTasks(ctx, tx, keys)
		if err != nil || len(cited) == 0 {
			return err
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
			if err := service.moveTask(ctx, tx, task, target, event, trigger); err != nil {
				return err
			}
		}
		return nil
	})
}

// lockCitedTasks locks the cited tasks' boards, then reads the tasks again so their columns are current under the lock.
func (service *PullRequestSyncService) lockCitedTasks(ctx context.Context, tx pgx.Tx, keys []TaskKey) ([]CitedTask, error) {
	cited, err := service.citedTasks.ListByKeys(ctx, tx, keys)
	if err != nil || len(cited) == 0 {
		return nil, err
	}
	if err := platform.LockBoards(ctx, tx, boardIDsOf(cited)); err != nil {
		return nil, err
	}
	return service.citedTasks.ListByKeys(ctx, tx, keys)
}

func (service *PullRequestSyncService) moveTask(ctx context.Context, tx pgx.Tx, task CitedTask, target *MappedColumn, event PullRequestEvent, trigger PullRequestTrigger) error {
	if err := service.mover.MoveTaskToColumnEnd(ctx, tx, task.TaskID, target.ID); err != nil {
		return err
	}
	return service.links.RecordMove(ctx, tx, TaskPullRequestMove{
		TaskID: task.TaskID, Repository: event.Repository, PullRequestNumber: event.Number, Trigger: trigger, At: service.now(),
	})
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
