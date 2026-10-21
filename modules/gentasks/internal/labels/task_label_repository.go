package labels

import (
	"context"
	"fmt"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
)

// TaskLabelRepository owns the task ↔ label association. Its rows are physically deleted (documented exception to rule 22).
type TaskLabelRepository struct{}

func (TaskLabelRepository) ReplaceForTask(ctx context.Context, db platform.DBTX, taskID int64, labelIDs []int64) error {
	if _, err := db.Exec(ctx, `DELETE FROM task_labels WHERE task_id = $1 AND NOT (label_id = ANY($2))`, taskID, labelIDs); err != nil {
		return fmt.Errorf("detach task labels: %w", err)
	}
	tokens := make([]string, len(labelIDs))
	for index := range labelIDs {
		tokens[index] = platform.NewPublicToken()
	}
	_, err := db.Exec(ctx, `
		INSERT INTO task_labels (token, task_id, label_id)
		SELECT requested.token, $1, requested.label_id
		FROM unnest($2::text[], $3::bigint[]) AS requested (token, label_id)
		ON CONFLICT (task_id, label_id) DO NOTHING`, taskID, tokens, labelIDs)
	if err != nil {
		return fmt.Errorf("attach task labels: %w", err)
	}
	return nil
}

func (TaskLabelRepository) DetachLabel(ctx context.Context, db platform.DBTX, labelID int64) error {
	if _, err := db.Exec(ctx, `DELETE FROM task_labels WHERE label_id = $1`, labelID); err != nil {
		return fmt.Errorf("detach label: %w", err)
	}
	return nil
}

// ListForTasks loads the active labels of many tasks in one query (rule 21), keyed by task ID.
func (TaskLabelRepository) ListForTasks(ctx context.Context, db platform.DBTX, taskIDs []int64) (map[int64][]Label, error) {
	rows, err := db.Query(ctx, `
		SELECT task_labels.task_id, `+labelColumns+`
		FROM task_labels JOIN labels ON labels.id = task_labels.label_id
		WHERE task_labels.task_id = ANY($1) AND labels.deleted_at IS NULL
		ORDER BY lower(labels.name), labels.id`, taskIDs)
	if err != nil {
		return nil, fmt.Errorf("list task labels: %w", err)
	}
	defer rows.Close()
	byTask := map[int64][]Label{}
	for rows.Next() {
		var taskID int64
		var label Label
		if err := rows.Scan(&taskID, &label.ID, &label.Token, &label.BoardID, &label.Name, &label.Color, &label.CreatedAt, &label.UpdatedAt); err != nil {
			return nil, fmt.Errorf("list task labels: %w", err)
		}
		byTask[taskID] = append(byTask[taskID], label)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list task labels: %w", err)
	}
	return byTask, nil
}
