package tasks

import (
	"context"
	"errors"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/boards"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/labels"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/coreclient"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TaskWithLabels is a task with its active labels, as every task response shows it.
type TaskWithLabels struct {
	Task
	Labels []labels.Label
}

// Caller is who makes the request; the access token is forwarded to core to check assignees with the caller's own rights.
type Caller struct {
	UserToken   string
	AccessToken string
}

type TaskService struct {
	db            *pgxpool.Pool
	boards        boards.BoardRepository
	columns       boards.ColumnRepository
	labels        labels.LabelRepository
	taskLabels    labels.TaskLabelRepository
	tasks         TaskRepository
	userDirectory UserDirectory
	now           func() time.Time
}

func NewTaskService(db *pgxpool.Pool, userDirectory UserDirectory, now func() time.Time) *TaskService {
	return &TaskService{db: db, userDirectory: userDirectory, now: now}
}

func (service *TaskService) CreateTask(ctx context.Context, caller Caller, boardToken string, input CreateTaskInput) (TaskWithLabels, error) {
	board, err := service.boards.FindByToken(ctx, service.db, boardToken)
	if err != nil {
		return TaskWithLabels{}, err
	}
	if err := service.verifyAssignee(ctx, caller, input.AssigneeToken); err != nil {
		return TaskWithLabels{}, err
	}
	var created TaskWithLabels
	err = pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		if err := platform.LockBoard(ctx, tx, board.ID); err != nil {
			return err
		}
		column, err := service.findTargetColumn(ctx, tx, board.ID, input.ColumnToken)
		if err != nil {
			return err
		}
		boardLabels, err := service.findBoardLabels(ctx, tx, board.ID, input.LabelTokens)
		if err != nil {
			return err
		}
		position, err := service.tasks.CountActiveInColumn(ctx, tx, column.ID)
		if err != nil {
			return err
		}
		task, err := service.tasks.Create(ctx, tx, NewTask{
			BoardID: board.ID, ColumnID: column.ID, Title: input.Title, Description: input.Description,
			AssigneeUserToken: input.AssigneeToken, Urgency: input.Urgency, Position: position,
			CreatedBy: caller.UserToken, At: service.now(),
		})
		if err != nil {
			return err
		}
		if err := service.taskLabels.ReplaceForTask(ctx, tx, task.ID, labelIDs(boardLabels)); err != nil {
			return err
		}
		created, err = service.loadWithLabels(ctx, tx, task.ID)
		return err
	})
	return created, err
}

func (service *TaskService) FindTask(ctx context.Context, token string) (TaskWithLabels, error) {
	task, err := service.tasks.FindByToken(ctx, service.db, token)
	if err != nil {
		return TaskWithLabels{}, err
	}
	return service.loadWithLabels(ctx, service.db, task.ID)
}

func (service *TaskService) ListTasks(ctx context.Context, boardToken string, archived bool) ([]TaskWithLabels, error) {
	board, err := service.boards.FindByToken(ctx, service.db, boardToken)
	if err != nil {
		return nil, err
	}
	var listed []Task
	if archived {
		listed, err = service.tasks.ListArchivedByBoard(ctx, service.db, board.ID)
	} else {
		listed, err = service.tasks.ListActiveByBoard(ctx, service.db, board.ID)
	}
	if err != nil {
		return nil, err
	}
	return service.AttachLabels(ctx, service.db, listed)
}

func (service *TaskService) UpdateTask(ctx context.Context, caller Caller, token string, changes TaskChanges) (TaskWithLabels, error) {
	if err := service.verifyAssignee(ctx, caller, changes.NewAssigneeToken()); err != nil {
		return TaskWithLabels{}, err
	}
	var updated TaskWithLabels
	err := service.withLockedTask(ctx, token, func(tx pgx.Tx, task Task) error {
		if changes.ReplacesLabels {
			boardLabels, err := service.findBoardLabels(ctx, tx, task.BoardID, changes.LabelTokens)
			if err != nil {
				return err
			}
			if err := service.taskLabels.ReplaceForTask(ctx, tx, task.ID, labelIDs(boardLabels)); err != nil {
				return err
			}
		}
		if err := service.tasks.UpdateDetails(ctx, tx, changes.ApplyTo(task), service.now()); err != nil {
			return err
		}
		var err error
		updated, err = service.loadWithLabels(ctx, tx, task.ID)
		return err
	})
	return updated, err
}

func (service *TaskService) MoveTask(ctx context.Context, token, columnToken string, targetPosition int) error {
	return service.withLockedTask(ctx, token, func(tx pgx.Tx, task Task) error {
		if task.IsArchived() {
			return ErrTaskArchived
		}
		target, err := service.columns.FindByToken(ctx, tx, columnToken)
		if errors.Is(err, boards.ErrColumnNotFound) || (err == nil && target.BoardID != task.BoardID) {
			return httpapi.NewValidationError(map[string]string{"columnToken": "invalid"})
		}
		if err != nil {
			return err
		}
		if target.ID == task.ColumnID {
			return service.reorderInColumn(ctx, tx, task, targetPosition)
		}
		return service.moveToColumn(ctx, tx, task, target.ID, targetPosition)
	})
}

func (service *TaskService) ArchiveTask(ctx context.Context, token string) error {
	return service.withLockedTask(ctx, token, func(tx pgx.Tx, task Task) error {
		if task.IsArchived() {
			return nil
		}
		if err := service.closeGap(ctx, tx, task); err != nil {
			return err
		}
		return service.tasks.Archive(ctx, tx, task.ID, service.now())
	})
}

func (service *TaskService) UnarchiveTask(ctx context.Context, token string) error {
	return service.withLockedTask(ctx, token, func(tx pgx.Tx, task Task) error {
		if !task.IsArchived() {
			return nil
		}
		last, err := service.tasks.CountActiveInColumn(ctx, tx, task.ColumnID)
		if err != nil {
			return err
		}
		return service.tasks.Place(ctx, tx, task.ID, task.ColumnID, last, service.now())
	})
}

func (service *TaskService) DeleteTask(ctx context.Context, token string) error {
	return service.withLockedTask(ctx, token, func(tx pgx.Tx, task Task) error {
		if !task.IsArchived() {
			if err := service.closeGap(ctx, tx, task); err != nil {
				return err
			}
		}
		return service.tasks.SoftDelete(ctx, tx, task.ID, service.now())
	})
}

// AttachLabels loads the labels of many tasks in one query (rule 21).
func (service *TaskService) AttachLabels(ctx context.Context, db platform.DBTX, listed []Task) ([]TaskWithLabels, error) {
	ids := make([]int64, 0, len(listed))
	for _, task := range listed {
		ids = append(ids, task.ID)
	}
	byTask, err := service.taskLabels.ListForTasks(ctx, db, ids)
	if err != nil {
		return nil, err
	}
	withLabels := make([]TaskWithLabels, 0, len(listed))
	for _, task := range listed {
		withLabels = append(withLabels, TaskWithLabels{Task: task, Labels: byTask[task.ID]})
	}
	return withLabels, nil
}

func (service *TaskService) reorderInColumn(ctx context.Context, tx pgx.Tx, task Task, targetPosition int) error {
	count, err := service.tasks.CountActiveInColumn(ctx, tx, task.ColumnID)
	if err != nil {
		return err
	}
	target := platform.ClampPosition(targetPosition, count-1)
	err = platform.ReorderWithin(*task.Position, target, func(from, to, delta int) error {
		return service.tasks.ShiftPositions(ctx, tx, task.ColumnID, from, to, delta)
	})
	if err != nil {
		return err
	}
	return service.tasks.Place(ctx, tx, task.ID, task.ColumnID, target, service.now())
}

func (service *TaskService) moveToColumn(ctx context.Context, tx pgx.Tx, task Task, columnID int64, targetPosition int) error {
	if err := service.closeGap(ctx, tx, task); err != nil {
		return err
	}
	count, err := service.tasks.CountActiveInColumn(ctx, tx, columnID)
	if err != nil {
		return err
	}
	target := platform.ClampPosition(targetPosition, count)
	if err := service.tasks.ShiftPositions(ctx, tx, columnID, target, platform.EndOfList, 1); err != nil {
		return err
	}
	return service.tasks.Place(ctx, tx, task.ID, columnID, target, service.now())
}

func (service *TaskService) closeGap(ctx context.Context, tx pgx.Tx, task Task) error {
	return service.tasks.ShiftPositions(ctx, tx, task.ColumnID, *task.Position+1, platform.EndOfList, -1)
}

// withLockedTask re-reads the task after locking its board, so the position it acts on is current.
func (service *TaskService) withLockedTask(ctx context.Context, token string, change func(tx pgx.Tx, task Task) error) error {
	return pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		task, err := service.tasks.FindByToken(ctx, tx, token)
		if err != nil {
			return err
		}
		if err := platform.LockBoard(ctx, tx, task.BoardID); err != nil {
			return err
		}
		task, err = service.tasks.FindByID(ctx, tx, task.ID)
		if err != nil {
			return err
		}
		return change(tx, task)
	})
}

func (service *TaskService) findTargetColumn(ctx context.Context, tx pgx.Tx, boardID int64, columnToken *string) (boards.Column, error) {
	if columnToken == nil {
		return service.columns.FindFirstInBoard(ctx, tx, boardID)
	}
	column, err := service.columns.FindByToken(ctx, tx, *columnToken)
	if errors.Is(err, boards.ErrColumnNotFound) || (err == nil && column.BoardID != boardID) {
		return boards.Column{}, httpapi.NewValidationError(map[string]string{"columnToken": "invalid"})
	}
	return column, err
}

func (service *TaskService) findBoardLabels(ctx context.Context, tx pgx.Tx, boardID int64, tokens []string) ([]labels.Label, error) {
	if len(tokens) == 0 {
		return nil, nil
	}
	found, err := service.labels.FindInBoardByTokens(ctx, tx, boardID, tokens)
	if err != nil {
		return nil, err
	}
	if len(found) != len(tokens) {
		return nil, httpapi.NewValidationError(map[string]string{"labelTokens": "invalid"})
	}
	return found, nil
}

func (service *TaskService) verifyAssignee(ctx context.Context, caller Caller, assigneeToken *string) error {
	if assigneeToken == nil {
		return nil
	}
	_, err := service.userDirectory.FindUser(ctx, caller.AccessToken, *assigneeToken)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, coreclient.ErrUserNotFound):
		return httpapi.NewValidationError(map[string]string{"assigneeToken": "unknown"})
	case errors.Is(err, coreclient.ErrDirectoryForbidden):
		return httpapi.ErrForbidden
	default:
		return ErrCoreUnavailable
	}
}

func (service *TaskService) loadWithLabels(ctx context.Context, db platform.DBTX, taskID int64) (TaskWithLabels, error) {
	task, err := service.tasks.FindByID(ctx, db, taskID)
	if err != nil {
		return TaskWithLabels{}, err
	}
	withLabels, err := service.AttachLabels(ctx, db, []Task{task})
	if err != nil {
		return TaskWithLabels{}, err
	}
	return withLabels[0], nil
}

func labelIDs(boardLabels []labels.Label) []int64 {
	ids := make([]int64, 0, len(boardLabels))
	for _, label := range boardLabels {
		ids = append(ids, label.ID)
	}
	return ids
}
