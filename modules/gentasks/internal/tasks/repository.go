package tasks

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
	"github.com/jackc/pgx/v5"
)

type TaskRepository struct{}

type NewTask struct {
	BoardID           int64
	Number            int
	ColumnID          int64
	Title             string
	Description       *string
	AssigneeUserToken *string
	AssignedAt        *time.Time
	Urgency           Urgency
	Position          int
	CreatedBy         string
	At                time.Time
}

const taskColumns = `tasks.id, tasks.token, tasks.number, tasks.board_id, tasks.column_id, board_columns.token, tasks.title, tasks.description,
       tasks.assignee_user_token, tasks.assigned_at, tasks.urgency, tasks.position, tasks.created_by, tasks.created_at, tasks.updated_at,
       tasks.archived_at`

const taskSelect = `
SELECT ` + taskColumns + `
FROM tasks
JOIN board_columns ON board_columns.id = tasks.column_id
JOIN boards ON boards.id = tasks.board_id`

func (repository TaskRepository) Create(ctx context.Context, db platform.DBTX, newTask NewTask) (Task, error) {
	var id int64
	err := db.QueryRow(ctx, `
		INSERT INTO tasks (token, number, board_id, column_id, title, description, assignee_user_token, assigned_at, urgency, position,
		                   created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $12)
		RETURNING id`,
		platform.NewPublicToken(), newTask.Number, newTask.BoardID, newTask.ColumnID, newTask.Title, newTask.Description,
		newTask.AssigneeUserToken, newTask.AssignedAt, newTask.Urgency, newTask.Position, newTask.CreatedBy, newTask.At).Scan(&id)
	if err != nil {
		return Task{}, fmt.Errorf("create task: %w", err)
	}
	return repository.FindByID(ctx, db, id)
}

// FindByToken finds active and archived tasks, but never deleted ones or tasks of a deleted board.
func (TaskRepository) FindByToken(ctx context.Context, db platform.DBTX, token string) (Task, error) {
	return scanSingleTask(db.QueryRow(ctx, taskSelect+`
		WHERE tasks.token = $1 AND tasks.deleted_at IS NULL AND boards.deleted_at IS NULL`, token))
}

func (TaskRepository) FindByID(ctx context.Context, db platform.DBTX, id int64) (Task, error) {
	return scanSingleTask(db.QueryRow(ctx, taskSelect+` WHERE tasks.id = $1 AND tasks.deleted_at IS NULL`, id))
}

func (TaskRepository) ListActiveByBoard(ctx context.Context, db platform.DBTX, boardID int64) ([]Task, error) {
	return listTasks(ctx, db, taskSelect+`
		WHERE tasks.board_id = $1 AND tasks.position IS NOT NULL
		ORDER BY board_columns.position, tasks.position`, boardID)
}

func (TaskRepository) ListArchivedByBoard(ctx context.Context, db platform.DBTX, boardID int64) ([]Task, error) {
	return listTasks(ctx, db, taskSelect+`
		WHERE tasks.board_id = $1 AND tasks.archived_at IS NOT NULL AND tasks.deleted_at IS NULL
		ORDER BY tasks.archived_at DESC, tasks.id DESC`, boardID)
}

// ListActiveAssignedTo lists a user's active tasks outside Done-preset columns, the longest-assigned first.
func (TaskRepository) ListActiveAssignedTo(ctx context.Context, db platform.DBTX, userToken string, limit int) ([]AssignedTask, error) {
	rows, err := db.Query(ctx, `
		SELECT `+taskColumns+`, boards.token, boards.name, boards.task_prefix, board_columns.preset, board_columns.name
		FROM tasks
		JOIN board_columns ON board_columns.id = tasks.column_id
		JOIN boards ON boards.id = tasks.board_id
		WHERE tasks.assignee_user_token = $1 AND tasks.position IS NOT NULL AND boards.deleted_at IS NULL
		  AND board_columns.preset IS DISTINCT FROM 'done'
		ORDER BY tasks.assigned_at, tasks.created_at, tasks.token
		LIMIT $2`, userToken, limit)
	if err != nil {
		return nil, fmt.Errorf("list assigned tasks: %w", err)
	}
	assigned, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (AssignedTask, error) {
		var task AssignedTask
		err := row.Scan(append(taskFields(&task.Task),
			&task.Board.Token, &task.Board.Name, &task.Board.TaskPrefix, &task.Column.Preset, &task.Column.Name)...)
		task.Column.Token = task.ColumnToken
		return task, err
	})
	if err != nil {
		return nil, fmt.Errorf("list assigned tasks: %w", err)
	}
	return assigned, nil
}

func (TaskRepository) CountActiveInColumn(ctx context.Context, db platform.DBTX, columnID int64) (int, error) {
	return countTasks(ctx, db, `SELECT count(*) FROM tasks WHERE column_id = $1 AND position IS NOT NULL`, columnID)
}

// CountInColumn counts active and archived tasks: both keep a column from being deleted.
func (TaskRepository) CountInColumn(ctx context.Context, db platform.DBTX, columnID int64) (int, error) {
	return countTasks(ctx, db, `SELECT count(*) FROM tasks WHERE column_id = $1 AND deleted_at IS NULL`, columnID)
}

// ShiftPositions moves every active task positioned in [from, to] of the column by delta.
func (TaskRepository) ShiftPositions(ctx context.Context, db platform.DBTX, columnID int64, from, to, delta int) error {
	_, err := db.Exec(ctx, `
		UPDATE tasks SET position = position + $4
		WHERE column_id = $1 AND position BETWEEN $2 AND $3`, columnID, from, to, delta)
	if err != nil {
		return fmt.Errorf("shift task positions: %w", err)
	}
	return nil
}

func (TaskRepository) Place(ctx context.Context, db platform.DBTX, taskID, columnID int64, position int, at time.Time) error {
	_, err := db.Exec(ctx, `
		UPDATE tasks SET column_id = $2, position = $3, archived_at = NULL, updated_at = $4
		WHERE id = $1`, taskID, columnID, position, at)
	if err != nil {
		return fmt.Errorf("place task: %w", err)
	}
	return nil
}

func (TaskRepository) UpdateDetails(ctx context.Context, db platform.DBTX, task Task, at time.Time) error {
	_, err := db.Exec(ctx, `
		UPDATE tasks SET title = $2, description = $3, assignee_user_token = $4, assigned_at = $5, urgency = $6, updated_at = $7
		WHERE id = $1`, task.ID, task.Title, task.Description, task.AssigneeUserToken, task.AssignedAt, task.Urgency, at)
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	return nil
}

func (TaskRepository) Archive(ctx context.Context, db platform.DBTX, taskID int64, at time.Time) error {
	_, err := db.Exec(ctx, `UPDATE tasks SET position = NULL, archived_at = $2, updated_at = $2 WHERE id = $1`, taskID, at)
	if err != nil {
		return fmt.Errorf("archive task: %w", err)
	}
	return nil
}

func (TaskRepository) SoftDelete(ctx context.Context, db platform.DBTX, taskID int64, at time.Time) error {
	_, err := db.Exec(ctx, `UPDATE tasks SET position = NULL, deleted_at = $2 WHERE id = $1`, taskID, at)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	return nil
}

func countTasks(ctx context.Context, db platform.DBTX, query string, columnID int64) (int, error) {
	var count int
	if err := db.QueryRow(ctx, query, columnID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count tasks: %w", err)
	}
	return count, nil
}

func listTasks(ctx context.Context, db platform.DBTX, query string, arguments ...any) ([]Task, error) {
	rows, err := db.Query(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	tasks, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Task, error) {
		return scanTask(row)
	})
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	return tasks, nil
}

func scanSingleTask(row pgx.Row) (Task, error) {
	task, err := scanTask(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrTaskNotFound
	}
	if err != nil {
		return Task{}, fmt.Errorf("find task: %w", err)
	}
	return task, nil
}

func scanTask(row pgx.Row) (Task, error) {
	var task Task
	err := row.Scan(taskFields(&task)...)
	return task, err
}

// taskFields lists the scan targets in the order of taskColumns.
func taskFields(task *Task) []any {
	return []any{&task.ID, &task.Token, &task.Number, &task.BoardID, &task.ColumnID, &task.ColumnToken, &task.Title, &task.Description,
		&task.AssigneeUserToken, &task.AssignedAt, &task.Urgency, &task.Position, &task.CreatedBy, &task.CreatedAt, &task.UpdatedAt, &task.ArchivedAt}
}
