package dbtest

import (
	"context"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DatabaseURL returns TEST_DATABASE_URL and fails the test when it is missing.
func DatabaseURL(t *testing.T) string {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("TEST_DATABASE_URL is required to run database tests")
	}
	return databaseURL
}

// NewSchemaName reserves a unique schema name and drops whatever was created under it when the test ends.
func NewSchemaName(t *testing.T) string {
	t.Helper()
	databaseURL := DatabaseURL(t)
	schema := "test_gentasks_" + strings.ToLower(hex.EncodeToString([]byte(platform.NewPublicToken()[:6])))
	t.Cleanup(func() { dropSchema(t, databaseURL, schema) })
	return schema
}

// NewMigratedSchema migrates a throwaway schema in TEST_DATABASE_URL and drops it when the test ends.
func NewMigratedSchema(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := DatabaseURL(t)
	schema := NewSchemaName(t)
	ctx := context.Background()

	if err := platform.Migrate(ctx, databaseURL, schema); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}
	pool, err := platform.OpenDatabase(ctx, databaseURL, schema)
	if err != nil {
		t.Fatalf("open test schema: %v", err)
	}
	t.Cleanup(pool.Close)
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
	if _, err := conn.Exec(ctx, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schema}.Sanitize()+" CASCADE"); err != nil {
		t.Errorf("drop schema %s: %v", schema, err)
	}
}
