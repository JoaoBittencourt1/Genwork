package platform_test

import (
	"context"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform/dbtest"
	"github.com/jackc/pgx/v5"
)

func TestMigrateCreatesTablesAndIsIdempotent(t *testing.T) {
	databaseURL := dbtest.DatabaseURL(t)
	schema := dbtest.NewSchemaName(t)
	ctx := context.Background()

	for range 2 {
		if err := platform.Migrate(ctx, databaseURL, schema); err != nil {
			t.Fatalf("migrate: %v", err)
		}
	}

	pool, err := platform.OpenDatabase(ctx, databaseURL, schema)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer pool.Close()
	for _, table := range []string{"boards", "board_columns", "labels", "tasks", "task_labels"} {
		var exists bool
		err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, pgx.Identifier{schema, table}.Sanitize()).Scan(&exists)
		if err != nil || !exists {
			t.Errorf("table %s missing (err %v)", table, err)
		}
	}
}

func TestMigrateUsesExistingSchema(t *testing.T) {
	databaseURL := dbtest.DatabaseURL(t)
	schema := dbtest.NewSchemaName(t)
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	if err := platform.Migrate(ctx, databaseURL, schema); err != nil {
		t.Fatalf("migrate into a provisioned schema: %v", err)
	}
}

func TestOpenDatabaseFailsOnBadURL(t *testing.T) {
	if _, err := platform.OpenDatabase(context.Background(), "not a url", "gentasks"); err == nil {
		t.Error("expected an error")
	}
	if _, err := platform.OpenDatabase(context.Background(), "postgres://nobody@127.0.0.1:1/none?connect_timeout=1", "gentasks"); err == nil {
		t.Error("expected a ping error")
	}
	if err := platform.Migrate(context.Background(), "not a url", "gentasks"); err == nil {
		t.Error("expected a migrate error")
	}
}
