package platform

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row
}

// NewPublicToken returns 128 random bits as base64url, the same format core uses (rule 14).
func NewPublicToken() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		panic(fmt.Sprintf("read random bytes: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(bytes)
}

func OpenDatabase(ctx context.Context, databaseURL, schema string) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse GENTASKS_DATABASE_URL: %w", err)
	}
	poolConfig.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// Migrate creates the schema only when it is missing: in an installation the CLI provisions it for the gentasks role, which may not create schemas.
func Migrate(ctx context.Context, databaseURL, schema string) error {
	pool, err := OpenDatabase(ctx, databaseURL, schema)
	if err != nil {
		return err
	}
	defer pool.Close()

	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = $1)`, schema).Scan(&exists); err != nil {
		return fmt.Errorf("check schema %s: %w", schema, err)
	}
	if !exists {
		if _, err := pool.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
			return fmt.Errorf("create schema %s: %w", schema, err)
		}
	}
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	return runMigrations(ctx, db)
}

func runMigrations(ctx context.Context, db *sql.DB) error {
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

const uniqueViolationCode = "23505"

func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode
}

// LockBoard serializes every write that changes the order of a board's columns or tasks (design §8).
func LockBoard(ctx context.Context, db DBTX, boardID int64) error {
	if _, err := db.Exec(ctx, `SELECT 1 FROM boards WHERE id = $1 FOR UPDATE`, boardID); err != nil {
		return fmt.Errorf("lock board: %w", err)
	}
	return nil
}
