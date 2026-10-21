package platform_test

import (
	"context"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform/dbtest"
)

const taskNumbersMigrationVersion = 2

func TestAssignedAtMigrationBackfillsFromCreationTime(t *testing.T) {
	databaseURL := dbtest.DatabaseURL(t)
	schema := dbtest.NewSchemaName(t)
	ctx := context.Background()
	pool := openSchemaAtMigration(t, databaseURL, schema, taskNumbersMigrationVersion)
	createdAt := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	_, err := pool.Exec(ctx, `
		INSERT INTO boards (token, name, task_prefix, created_by, created_at, updated_at) VALUES ('b1', 'Board', 'BOA', 'u', '2026-09-01T09:00:00Z', now());
		INSERT INTO board_columns (token, board_id, preset, position, created_at, updated_at) VALUES ('c1', 1, 'backlog', 0, now(), now());
		INSERT INTO tasks (token, number, board_id, column_id, title, assignee_user_token, position, created_by, created_at, updated_at) VALUES
			('assigned', 1, 1, 1, 'Assigned', 'ana', 0, 'u', '2026-09-01T09:00:00Z', '2026-09-06T09:00:00Z'),
			('unassigned', 2, 1, 1, 'Unassigned', NULL, 1, 'u', '2026-09-01T09:00:00Z', now());`)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := platform.Migrate(ctx, databaseURL, schema); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var assigned, unassigned *time.Time
	if err := pool.QueryRow(ctx, `SELECT assigned_at FROM tasks WHERE token = 'assigned'`).Scan(&assigned); err != nil || assigned == nil || !assigned.Equal(createdAt) {
		t.Errorf("assigned task assigned_at = %v, err = %v", assigned, err)
	}
	if err := pool.QueryRow(ctx, `SELECT assigned_at FROM tasks WHERE token = 'unassigned'`).Scan(&unassigned); err != nil || unassigned != nil {
		t.Errorf("unassigned task assigned_at = %v, err = %v", unassigned, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE tasks SET assigned_at = NULL WHERE token = 'assigned'`); err == nil {
		t.Error("an assignee without an assignment time must be rejected")
	}
}
