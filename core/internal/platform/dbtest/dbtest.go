package dbtest

import (
	"context"
	"encoding/hex"
	"os"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewMigratedSchema migrates a throwaway schema in TEST_DATABASE_URL and drops it when the test ends.
func NewMigratedSchema(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("TEST_DATABASE_URL is required to run database tests")
	}
	ctx := context.Background()
	schema := "test_" + hex.EncodeToString(platform.RandomBytes(6))

	if err := platform.Migrate(ctx, databaseURL, schema); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}
	pool, err := platform.OpenDatabase(ctx, databaseURL, schema)
	if err != nil {
		t.Fatalf("open test schema: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		dropSchema(t, databaseURL, schema)
	})
	return pool
}

func dropSchema(t *testing.T, databaseURL, schema string) {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Errorf("connect to drop schema %s: %v", schema, err)
		return
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE"); err != nil {
		t.Errorf("drop schema %s: %v", schema, err)
	}
}
