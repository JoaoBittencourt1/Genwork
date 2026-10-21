package platform_test

import (
	"context"
	"slices"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform/dbtest"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

const firstMigrationVersion = 1

func TestTaskNumbersMigrationBackfillsExistingBoards(t *testing.T) {
	databaseURL := dbtest.DatabaseURL(t)
	schema := dbtest.NewSchemaName(t)
	ctx := context.Background()
	pool := openSchemaAtFirstMigration(t, databaseURL, schema)
	seedBoardsBeforeTaskNumbers(t, pool)

	if err := platform.Migrate(ctx, databaseURL, schema); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	rows, _ := pool.Query(ctx, `SELECT task_prefix FROM boards ORDER BY id`)
	prefixes, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil || !slices.Equal(prefixes, []string{"PLA", "PLA2", "TASK"}) {
		t.Errorf("prefixes = %v, err = %v", prefixes, err)
	}
	rows, _ = pool.Query(ctx, `SELECT number FROM tasks ORDER BY id`)
	numbers, err := pgx.CollectRows(rows, pgx.RowTo[int])
	if err != nil || !slices.Equal(numbers, []int{2, 1, 1}) {
		t.Errorf("numbers = %v, err = %v", numbers, err)
	}
	var next int
	if err := pool.QueryRow(ctx, `SELECT next_task_number FROM boards WHERE task_prefix = 'PLA'`).Scan(&next); err != nil || next != 3 {
		t.Errorf("next number = %d, err = %v", next, err)
	}
}

func openSchemaAtFirstMigration(t *testing.T, databaseURL, schema string) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	pool, err := platform.OpenDatabase(ctx, databaseURL, schema)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(pool.Close)
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	if _, err := provider.UpTo(ctx, firstMigrationVersion); err != nil {
		t.Fatalf("migrate to the first version: %v", err)
	}
	return pool
}

// seedBoardsBeforeTaskNumbers: two boards whose names share a prefix, one without letters; the first board's tasks were created out of id order.
func seedBoardsBeforeTaskNumbers(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO boards (token, name, created_by, created_at, updated_at) VALUES
			('b1', 'Platform', 'u', now(), now()),
			('b2', 'Plataforma', 'u', now(), now()),
			('b3', '123', 'u', now(), now());
		INSERT INTO board_columns (token, board_id, preset, position, created_at, updated_at) VALUES
			('c1', 1, 'backlog', 0, now(), now()),
			('c2', 2, 'backlog', 0, now(), now());
		INSERT INTO tasks (token, board_id, column_id, title, position, created_by, created_at, updated_at) VALUES
			('t1', 1, 1, 'Later', 0, 'u', now(), now()),
			('t2', 1, 1, 'Earlier', 1, 'u', now() - interval '1 day', now()),
			('t3', 2, 2, 'Other board', 0, 'u', now(), now());`)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
}
