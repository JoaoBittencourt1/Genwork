package runners

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform"
	"github.com/jackc/pgx/v5"
)

type RunnerRepository struct{}

const runnerColumns = `id, token, name, secret_hash IS NOT NULL, last_heartbeat_at, container_engine, sandbox_image, directory_mode,
       reported_tools, has_github_credentials, max_concurrency, revoked_at, created_at`

func (repository RunnerRepository) Create(ctx context.Context, db platform.DBTX, name string, registrationTokenHash []byte, at time.Time) (Runner, error) {
	var id int64
	err := db.QueryRow(ctx, `
		INSERT INTO runners (token, name, registration_token_hash, registration_expires_at, max_concurrency, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		RETURNING id`,
		platform.NewPublicToken(), name, registrationTokenHash, at.Add(RegistrationLifetime), DefaultConcurrency, at).Scan(&id)
	if platform.IsUniqueViolation(err) {
		return Runner{}, ErrRunnerNameTaken
	}
	if err != nil {
		return Runner{}, fmt.Errorf("create runner: %w", err)
	}
	return repository.FindByID(ctx, db, id)
}

func (RunnerRepository) FindByID(ctx context.Context, db platform.DBTX, id int64) (Runner, error) {
	return scanSingleRunner(db.QueryRow(ctx, `SELECT `+runnerColumns+` FROM runners WHERE id = $1 AND deleted_at IS NULL`, id))
}

func (RunnerRepository) FindByToken(ctx context.Context, db platform.DBTX, token string) (Runner, error) {
	return scanSingleRunner(db.QueryRow(ctx, `SELECT `+runnerColumns+` FROM runners WHERE token = $1 AND deleted_at IS NULL`, token))
}

func (RunnerRepository) FindBySecretHash(ctx context.Context, db platform.DBTX, secretHash []byte) (Runner, error) {
	runner, err := scanSingleRunner(db.QueryRow(ctx, `SELECT `+runnerColumns+` FROM runners WHERE secret_hash = $1 AND deleted_at IS NULL`, secretHash))
	if errors.Is(err, ErrRunnerNotFound) {
		return Runner{}, ErrInvalidRunnerSecret
	}
	return runner, err
}

func (RunnerRepository) List(ctx context.Context, db platform.DBTX) ([]Runner, error) {
	rows, err := db.Query(ctx, `SELECT `+runnerColumns+` FROM runners WHERE deleted_at IS NULL ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list runners: %w", err)
	}
	listed, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Runner, error) { return scanRunner(row) })
	if err != nil {
		return nil, fmt.Errorf("list runners: %w", err)
	}
	return listed, nil
}

// ExchangeRegistration atomically consumes a registration token: it is valid once, for an hour, and for a runner that
// is not revoked. The token hash is cleared in the same statement, so it can never match again.
func (repository RunnerRepository) ExchangeRegistration(ctx context.Context, db platform.DBTX, registrationTokenHash, secretHash []byte, at time.Time) (Runner, error) {
	var id int64
	err := db.QueryRow(ctx, `
		UPDATE runners SET secret_hash = $2, registration_token_hash = NULL, registration_expires_at = NULL, updated_at = $3
		WHERE registration_token_hash = $1 AND registration_expires_at > $3 AND revoked_at IS NULL AND deleted_at IS NULL
		RETURNING id`, registrationTokenHash, secretHash, at).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Runner{}, ErrInvalidRegistrationToken
	}
	if err != nil {
		return Runner{}, fmt.Errorf("exchange registration token: %w", err)
	}
	return repository.FindByID(ctx, db, id)
}

func (RunnerRepository) RecordHeartbeat(ctx context.Context, db platform.DBTX, id int64, heartbeat Heartbeat) error {
	tools, err := json.Marshal(heartbeat.Tools)
	if err != nil {
		return fmt.Errorf("encode reported tools: %w", err)
	}
	_, err = db.Exec(ctx, `
		UPDATE runners SET last_heartbeat_at = $2, container_engine = $3, sandbox_image = $4, directory_mode = $5, reported_tools = $6,
		                   has_github_credentials = $7, max_concurrency = $8, updated_at = $2
		WHERE id = $1`,
		id, heartbeat.At, nullIfEmpty(heartbeat.ContainerEngine), nullIfEmpty(heartbeat.SandboxImage), heartbeat.DirectoryMode, tools,
		heartbeat.HasGithubCredentials, heartbeat.MaxConcurrency)
	if err != nil {
		return fmt.Errorf("record heartbeat: %w", err)
	}
	return nil
}

// Revoke rejects the secret from now on: authentication looks the runner up by the secret hash of a non-revoked runner.
func (RunnerRepository) Revoke(ctx context.Context, db platform.DBTX, id int64, at time.Time) error {
	tag, err := db.Exec(ctx, `
		UPDATE runners SET revoked_at = $2, secret_hash = NULL, registration_token_hash = NULL, registration_expires_at = NULL, updated_at = $2
		WHERE id = $1 AND revoked_at IS NULL AND deleted_at IS NULL`, id, at)
	if err != nil {
		return fmt.Errorf("revoke runner: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrRunnerNotFound
	}
	return nil
}

func nullIfEmpty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func scanSingleRunner(row pgx.Row) (Runner, error) {
	runner, err := scanRunner(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Runner{}, ErrRunnerNotFound
	}
	if err != nil {
		return Runner{}, fmt.Errorf("find runner: %w", err)
	}
	return runner, nil
}

func scanRunner(row pgx.Row) (Runner, error) {
	var runner Runner
	var tools []byte
	err := row.Scan(&runner.ID, &runner.Token, &runner.Name, &runner.HasSecret, &runner.LastHeartbeatAt, &runner.ContainerEngine,
		&runner.SandboxImage, &runner.DirectoryMode, &tools, &runner.HasGithubCredentials, &runner.MaxConcurrency, &runner.RevokedAt, &runner.CreatedAt)
	if err != nil {
		return runner, err
	}
	if err := json.Unmarshal(tools, &runner.ReportedTools); err != nil {
		return runner, fmt.Errorf("decode reported tools: %w", err)
	}
	return runner, nil
}
