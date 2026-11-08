package mcpservers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform"
	"github.com/jackc/pgx/v5"
)

type McpServerOAuthRepository struct{}

const connectionColumns = `id, mcp_server_id, status, issuer, authorization_endpoint, token_endpoint, resource, scope, client_id,
       client_secret_ciphertext, token_endpoint_auth_method, access_token_ciphertext, refresh_token_ciphertext, expires_at,
       connected_at, last_error, created_at, updated_at`

// OAuthClientSetup is what starting an authorization stores: where to send the person and which client to be.
type OAuthClientSetup struct {
	Issuer                  string
	AuthorizationEndpoint   string
	TokenEndpoint           string
	Resource                string
	Scope                   string
	ClientID                string
	ClientSecretCiphertext  []byte
	TokenEndpointAuthMethod string
}

// SaveClientSetup stores the client of a new authorization; a connected server keeps its tokens and status until the new
// authorization succeeds.
func (McpServerOAuthRepository) SaveClientSetup(ctx context.Context, db platform.DBTX, serverID int64, setup OAuthClientSetup, at time.Time) error {
	_, err := db.Exec(ctx, `
		INSERT INTO mcp_server_oauth_connections (mcp_server_id, status, issuer, authorization_endpoint, token_endpoint, resource, scope,
		       client_id, client_secret_ciphertext, token_endpoint_auth_method, created_at, updated_at)
		VALUES ($1, 'pending', $2, $3, $4, $5, NULLIF($6, ''), $7, $8, NULLIF($9, ''), $10, $10)
		ON CONFLICT (mcp_server_id) DO UPDATE SET
		       status = CASE WHEN mcp_server_oauth_connections.status = 'connected' THEN 'connected' ELSE 'pending' END,
		       issuer = EXCLUDED.issuer, authorization_endpoint = EXCLUDED.authorization_endpoint, token_endpoint = EXCLUDED.token_endpoint,
		       resource = EXCLUDED.resource, scope = EXCLUDED.scope, client_id = EXCLUDED.client_id,
		       client_secret_ciphertext = EXCLUDED.client_secret_ciphertext, token_endpoint_auth_method = EXCLUDED.token_endpoint_auth_method,
		       updated_at = EXCLUDED.updated_at`,
		serverID, setup.Issuer, setup.AuthorizationEndpoint, setup.TokenEndpoint, setup.Resource, setup.Scope, setup.ClientID,
		setup.ClientSecretCiphertext, setup.TokenEndpointAuthMethod, at)
	if err != nil {
		return fmt.Errorf("save oauth client: %w", err)
	}
	return nil
}

func (McpServerOAuthRepository) FindByServer(ctx context.Context, db platform.DBTX, serverID int64) (OAuthConnection, error) {
	connection, err := scanConnection(db.QueryRow(ctx, `SELECT `+connectionColumns+` FROM mcp_server_oauth_connections WHERE mcp_server_id = $1`, serverID))
	if errors.Is(err, pgx.ErrNoRows) {
		return OAuthConnection{}, ErrOAuthNotConnected
	}
	if err != nil {
		return OAuthConnection{}, fmt.Errorf("find oauth connection: %w", err)
	}
	return connection, nil
}

// ListByServers loads the connections of many servers in one query (rule 21).
func (McpServerOAuthRepository) ListByServers(ctx context.Context, db platform.DBTX, serverIDs []int64) (map[int64]OAuthConnection, error) {
	connections := map[int64]OAuthConnection{}
	if len(serverIDs) == 0 {
		return connections, nil
	}
	rows, err := db.Query(ctx, `SELECT `+connectionColumns+` FROM mcp_server_oauth_connections WHERE mcp_server_id = ANY($1)`, serverIDs)
	if err != nil {
		return nil, fmt.Errorf("list oauth connections: %w", err)
	}
	listed, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (OAuthConnection, error) { return scanConnection(row) })
	if err != nil {
		return nil, fmt.Errorf("list oauth connections: %w", err)
	}
	for _, connection := range listed {
		connections[connection.ServerID] = connection
	}
	return connections, nil
}

// OAuthTokenUpdate stores new tokens; a nil refresh ciphertext keeps the stored one, since servers may not rotate it.
type OAuthTokenUpdate struct {
	AccessTokenCiphertext  []byte
	RefreshTokenCiphertext []byte
	ExpiresAt              *time.Time
	At                     time.Time
}

func (McpServerOAuthRepository) StoreTokens(ctx context.Context, db platform.DBTX, serverID int64, update OAuthTokenUpdate) error {
	_, err := db.Exec(ctx, `
		UPDATE mcp_server_oauth_connections SET status = 'connected', access_token_ciphertext = $2,
		       refresh_token_ciphertext = COALESCE($3, refresh_token_ciphertext), expires_at = $4,
		       connected_at = COALESCE(connected_at, $5), last_error = NULL, updated_at = $5
		WHERE mcp_server_id = $1`, serverID, update.AccessTokenCiphertext, update.RefreshTokenCiphertext, update.ExpiresAt, update.At)
	if err != nil {
		return fmt.Errorf("store oauth tokens: %w", err)
	}
	return nil
}

func (McpServerOAuthRepository) MarkError(ctx context.Context, db platform.DBTX, serverID int64, message string, at time.Time) error {
	_, err := db.Exec(ctx, `UPDATE mcp_server_oauth_connections SET status = 'error', last_error = $2, updated_at = $3 WHERE mcp_server_id = $1`,
		serverID, truncateOAuthError(message), at)
	if err != nil {
		return fmt.Errorf("mark oauth error: %w", err)
	}
	return nil
}

// Delete removes the connection with every token in it: a disconnected server keeps no secret (rule 67).
func (McpServerOAuthRepository) Delete(ctx context.Context, db platform.DBTX, serverID int64) error {
	tag, err := db.Exec(ctx, `DELETE FROM mcp_server_oauth_connections WHERE mcp_server_id = $1`, serverID)
	if err != nil {
		return fmt.Errorf("delete oauth connection: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrOAuthNotConnected
	}
	return nil
}

func (McpServerOAuthRepository) CreateAuthorization(ctx context.Context, db platform.DBTX, token string, serverID int64, stateHash, verifierCiphertext []byte, expiresAt, at time.Time) error {
	_, err := db.Exec(ctx, `
		INSERT INTO mcp_server_oauth_authorizations (token, mcp_server_id, state_hash, code_verifier_ciphertext, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`, token, serverID, stateHash, verifierCiphertext, expiresAt, at)
	if err != nil {
		return fmt.Errorf("create oauth authorization: %w", err)
	}
	return nil
}

// ConsumeAuthorization marks an unexpired, unused authorization as used and returns it, so a state works exactly once.
func (McpServerOAuthRepository) ConsumeAuthorization(ctx context.Context, db platform.DBTX, stateHash []byte, at time.Time) (OAuthAuthorization, error) {
	var authorization OAuthAuthorization
	err := db.QueryRow(ctx, `
		UPDATE mcp_server_oauth_authorizations SET consumed_at = $2
		WHERE state_hash = $1 AND consumed_at IS NULL AND expires_at > $2
		RETURNING id, token, mcp_server_id, code_verifier_ciphertext, expires_at, consumed_at`, stateHash, at).
		Scan(&authorization.ID, &authorization.Token, &authorization.ServerID, &authorization.CodeVerifierCiphertext, &authorization.ExpiresAt, &authorization.ConsumedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return OAuthAuthorization{}, ErrOAuthStateInvalid
	}
	if err != nil {
		return OAuthAuthorization{}, fmt.Errorf("consume oauth authorization: %w", err)
	}
	return authorization, nil
}

func scanConnection(row pgx.Row) (OAuthConnection, error) {
	var connection OAuthConnection
	err := row.Scan(&connection.ID, &connection.ServerID, &connection.Status, &connection.Issuer, &connection.AuthorizationEndpoint,
		&connection.TokenEndpoint, &connection.Resource, &connection.Scope, &connection.ClientID, &connection.ClientSecretCiphertext,
		&connection.TokenEndpointAuthMethod, &connection.AccessTokenCiphertext, &connection.RefreshTokenCiphertext, &connection.ExpiresAt,
		&connection.ConnectedAt, &connection.LastError, &connection.CreatedAt, &connection.UpdatedAt)
	return connection, err
}
