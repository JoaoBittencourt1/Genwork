package githubsync

import (
	"context"
	"fmt"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
	"github.com/jackc/pgx/v5"
)

// CitedTask is an active task a pull request cites, with the position of the column it is in.
type CitedTask struct {
	TaskID         int64
	BoardID        int64
	ColumnID       int64
	ColumnPosition int
}

type CitedTaskRepository struct{}

// ListByKeys resolves every key in one query (rule 21); keys of unknown boards, missing numbers, and archived or deleted
// tasks yield nothing.
func (CitedTaskRepository) ListByKeys(ctx context.Context, db platform.DBTX, keys []TaskKey) ([]CitedTask, error) {
	prefixes, numbers := taskKeyColumns(keys)
	rows, err := db.Query(ctx, `
		SELECT tasks.id, tasks.board_id, tasks.column_id, board_columns.position
		FROM unnest($1::text[], $2::integer[]) AS cited (prefix, number)
		JOIN boards ON boards.task_prefix = cited.prefix AND boards.deleted_at IS NULL
		JOIN tasks ON tasks.board_id = boards.id AND tasks.number = cited.number
		JOIN board_columns ON board_columns.id = tasks.column_id
		WHERE tasks.deleted_at IS NULL AND tasks.archived_at IS NULL
		ORDER BY tasks.id`, prefixes, numbers)
	if err != nil {
		return nil, fmt.Errorf("list cited tasks: %w", err)
	}
	cited, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (CitedTask, error) {
		var task CitedTask
		err := row.Scan(&task.TaskID, &task.BoardID, &task.ColumnID, &task.ColumnPosition)
		return task, err
	})
	if err != nil {
		return nil, fmt.Errorf("list cited tasks: %w", err)
	}
	return cited, nil
}
