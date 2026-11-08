package mcpservers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform"
	"github.com/jackc/pgx/v5"
)

// StoredMcpServer is a row as stored: the clear config and the encrypted secrets.
type StoredMcpServer struct {
	McpServer
	SecretsCiphertext []byte
}

type McpServerRepository struct{}

const serverColumns = `id, token, name, transport, config, secrets_ciphertext, created_at, updated_at`

func (repository McpServerRepository) Create(ctx context.Context, db platform.DBTX, token, name string, transport Transport, config Config, ciphertext []byte, at time.Time) (StoredMcpServer, error) {
	encoded, err := json.Marshal(config)
	if err != nil {
		return StoredMcpServer{}, fmt.Errorf("encode mcp server config: %w", err)
	}
	_, err = db.Exec(ctx, `
		INSERT INTO mcp_servers (token, name, transport, config, secrets_ciphertext, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $6)`,
		token, name, transport, encoded, ciphertext, at)
	if platform.IsUniqueViolation(err) {
		return StoredMcpServer{}, ErrMCPServerNameTaken
	}
	if err != nil {
		return StoredMcpServer{}, fmt.Errorf("create mcp server: %w", err)
	}
	return repository.FindByToken(ctx, db, token)
}

func (repository McpServerRepository) Update(ctx context.Context, db platform.DBTX, id int64, name string, config Config, ciphertext []byte, at time.Time) error {
	encoded, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("encode mcp server config: %w", err)
	}
	tag, err := db.Exec(ctx, `
		UPDATE mcp_servers SET name = $2, config = $3, secrets_ciphertext = $4, updated_at = $5 WHERE id = $1 AND deleted_at IS NULL`,
		id, name, encoded, ciphertext, at)
	if platform.IsUniqueViolation(err) {
		return ErrMCPServerNameTaken
	}
	if err != nil {
		return fmt.Errorf("update mcp server: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrMCPServerNotFound
	}
	return nil
}

func (McpServerRepository) FindByToken(ctx context.Context, db platform.DBTX, token string) (StoredMcpServer, error) {
	stored, err := scanServer(db.QueryRow(ctx, `SELECT `+serverColumns+` FROM mcp_servers WHERE token = $1 AND deleted_at IS NULL`, token))
	if errors.Is(err, pgx.ErrNoRows) {
		return StoredMcpServer{}, ErrMCPServerNotFound
	}
	if err != nil {
		return StoredMcpServer{}, fmt.Errorf("find mcp server: %w", err)
	}
	return stored, nil
}

func (McpServerRepository) FindByID(ctx context.Context, db platform.DBTX, id int64) (StoredMcpServer, error) {
	stored, err := scanServer(db.QueryRow(ctx, `SELECT `+serverColumns+` FROM mcp_servers WHERE id = $1 AND deleted_at IS NULL`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return StoredMcpServer{}, ErrMCPServerNotFound
	}
	if err != nil {
		return StoredMcpServer{}, fmt.Errorf("find mcp server: %w", err)
	}
	return stored, nil
}

func (McpServerRepository) List(ctx context.Context, db platform.DBTX) ([]StoredMcpServer, error) {
	return listServers(ctx, db, `SELECT `+serverColumns+` FROM mcp_servers WHERE deleted_at IS NULL ORDER BY name`)
}

func (McpServerRepository) ListByIDs(ctx context.Context, db platform.DBTX, ids []int64) ([]StoredMcpServer, error) {
	return listServers(ctx, db, `SELECT `+serverColumns+` FROM mcp_servers WHERE id = ANY($1) AND deleted_at IS NULL ORDER BY name`, ids)
}

// SoftDelete clears the secrets in the same statement and detaches the server from every bot (rules 22 and 67).
func (McpServerRepository) SoftDelete(ctx context.Context, db platform.DBTX, id int64, at time.Time) error {
	if _, err := db.Exec(ctx, `DELETE FROM bot_mcp_servers WHERE mcp_server_id = $1`, id); err != nil {
		return fmt.Errorf("detach mcp server: %w", err)
	}
	tag, err := db.Exec(ctx, `UPDATE mcp_servers SET deleted_at = $2, updated_at = $2, secrets_ciphertext = NULL WHERE id = $1 AND deleted_at IS NULL`, id, at)
	if err != nil {
		return fmt.Errorf("delete mcp server: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrMCPServerNotFound
	}
	return nil
}

func listServers(ctx context.Context, db platform.DBTX, query string, arguments ...any) ([]StoredMcpServer, error) {
	rows, err := db.Query(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("list mcp servers: %w", err)
	}
	listed, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (StoredMcpServer, error) { return scanServer(row) })
	if err != nil {
		return nil, fmt.Errorf("list mcp servers: %w", err)
	}
	return listed, nil
}

func scanServer(row pgx.Row) (StoredMcpServer, error) {
	var stored StoredMcpServer
	var config []byte
	err := row.Scan(&stored.ID, &stored.Token, &stored.Name, &stored.Transport, &config, &stored.SecretsCiphertext, &stored.CreatedAt, &stored.UpdatedAt)
	if err != nil {
		return stored, err
	}
	return stored, json.Unmarshal(config, &stored.Config)
}
