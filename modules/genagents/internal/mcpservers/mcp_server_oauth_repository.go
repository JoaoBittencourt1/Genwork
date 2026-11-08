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

const connectionColumns = `id, mcp_server_id, status, server_url, token_endpoint, resource, client_id, client_secret_ciphertext,
       token_endpoint_auth_method, access_token_ciphertext, refresh_token_ciphertext, expires_at, connected_at, last_error,
       created_at, updated_at`

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

// OAuthTokenUpdate holds new tokens; a nil refresh ciphertext keeps the stored one, since servers may not rotate it.
type OAuthTokenUpdate struct {
	AccessTokenCiphertext  []byte
	RefreshTokenCiphertext []byte
	ExpiresAt              *time.Time
	At                     time.Time
}

// Connect makes the setup and tokens of a successful authorization the server's active connection, in one statement,
// so a run never pairs one client's refresh token with another client.
func (McpServerOAuthRepository) Connect(ctx context.Context, db platform.DBTX, serverID int64, setup OAuthClientSetup, tokens OAuthTokenUpdate) error {
	_, err := db.Exec(ctx, `
		INSERT INTO mcp_server_oauth_connections (mcp_server_id, status, server_url, token_endpoint, resource, client_id,
		       client_secret_ciphertext, token_endpoint_auth_method, access_token_ciphertext, refresh_token_ciphertext, expires_at,
		       connected_at, created_at, updated_at)
		VALUES ($1, 'connected', $2, $3, $4, $5, $6, NULLIF($7, ''), $8, $9, $10, $11, $11, $11)
		ON CONFLICT (mcp_server_id) DO UPDATE SET status = 'connected', server_url = EXCLUDED.server_url,
		       token_endpoint = EXCLUDED.token_endpoint, resource = EXCLUDED.resource, client_id = EXCLUDED.client_id,
		       client_secret_ciphertext = EXCLUDED.client_secret_ciphertext, token_endpoint_auth_method = EXCLUDED.token_endpoint_auth_method,
		       access_token_ciphertext = EXCLUDED.access_token_ciphertext, refresh_token_ciphertext = EXCLUDED.refresh_token_ciphertext,
		       expires_at = EXCLUDED.expires_at, connected_at = EXCLUDED.connected_at, last_error = NULL, updated_at = EXCLUDED.updated_at`,
		serverID, setup.ServerURL, setup.TokenEndpoint, setup.Resource, setup.ClientID, setup.ClientSecretCiphertext,
		setup.TokenEndpointAuthMethod, tokens.AccessTokenCiphertext, tokens.RefreshTokenCiphertext, tokens.ExpiresAt, tokens.At)
	if err != nil {
		return fmt.Errorf("store oauth connection: %w", err)
	}
	return nil
}

// RefreshTokens stores the tokens a refresh returned for the active connection.
func (McpServerOAuthRepository) RefreshTokens(ctx context.Context, db platform.DBTX, serverID int64, tokens OAuthTokenUpdate) error {
	_, err := db.Exec(ctx, `
		UPDATE mcp_server_oauth_connections SET access_token_ciphertext = $2, refresh_token_ciphertext = COALESCE($3, refresh_token_ciphertext),
		       expires_at = $4, last_error = NULL, updated_at = $5
		WHERE mcp_server_id = $1 AND status = 'connected'`, serverID, tokens.AccessTokenCiphertext, tokens.RefreshTokenCiphertext, tokens.ExpiresAt, tokens.At)
	if err != nil {
		return fmt.Errorf("refresh oauth tokens: %w", err)
	}
	return nil
}

// MarkError records why the active connection stopped working.
func (McpServerOAuthRepository) MarkError(ctx context.Context, db platform.DBTX, serverID int64, message string, at time.Time) error {
	_, err := db.Exec(ctx, `UPDATE mcp_server_oauth_connections SET status = 'error', last_error = $2, updated_at = $3 WHERE mcp_server_id = $1`,
		serverID, truncateOAuthError(message), at)
	if err != nil {
		return fmt.Errorf("mark oauth error: %w", err)
	}
	return nil
}

// RecordFailedAttempt shows a failed round trip on a server that has no working connection; a working one is kept,
// since a reconnection the person abandoned must not break it.
func (McpServerOAuthRepository) RecordFailedAttempt(ctx context.Context, db platform.DBTX, serverID int64, message string, at time.Time) error {
	_, err := db.Exec(ctx, `
		INSERT INTO mcp_server_oauth_connections (mcp_server_id, status, last_error, created_at, updated_at) VALUES ($1, 'error', $2, $3, $3)
		ON CONFLICT (mcp_server_id) DO UPDATE SET last_error = EXCLUDED.last_error, updated_at = EXCLUDED.updated_at
		WHERE mcp_server_oauth_connections.status <> 'connected'`, serverID, truncateOAuthError(message), at)
	if err != nil {
		return fmt.Errorf("record oauth failure: %w", err)
	}
	return nil
}

// Delete removes the connection with every token in it: a disconnected server keeps no secret (rule 67).
func (McpServerOAuthRepository) Delete(ctx context.Context, db platform.DBTX, serverID int64) (bool, error) {
	tag, err := db.Exec(ctx, `DELETE FROM mcp_server_oauth_connections WHERE mcp_server_id = $1`, serverID)
	if err != nil {
		return false, fmt.Errorf("delete oauth connection: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (McpServerOAuthRepository) CreateAuthorization(ctx context.Context, db platform.DBTX, authorization OAuthAuthorization, stateHash []byte, at time.Time) error {
	setup := authorization.Setup
	_, err := db.Exec(ctx, `
		INSERT INTO mcp_server_oauth_authorizations (token, mcp_server_id, state_hash, code_verifier_ciphertext, server_url, token_endpoint,
		       resource, client_id, client_secret_ciphertext, token_endpoint_auth_method, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NULLIF($10, ''), $11, $12)`,
		authorization.Token, authorization.ServerID, stateHash, authorization.CodeVerifierCiphertext, setup.ServerURL, setup.TokenEndpoint,
		setup.Resource, setup.ClientID, setup.ClientSecretCiphertext, setup.TokenEndpointAuthMethod, authorization.ExpiresAt, at)
	if err != nil {
		return fmt.Errorf("create oauth authorization: %w", err)
	}
	return nil
}

// ConsumeAuthorization marks an unexpired, unused authorization as used and returns it, so a state works exactly once.
func (McpServerOAuthRepository) ConsumeAuthorization(ctx context.Context, db platform.DBTX, stateHash []byte, at time.Time) (OAuthAuthorization, error) {
	var authorization OAuthAuthorization
	var authMethod *string
	setup := &authorization.Setup
	err := db.QueryRow(ctx, `
		UPDATE mcp_server_oauth_authorizations SET consumed_at = $2
		WHERE state_hash = $1 AND consumed_at IS NULL AND expires_at > $2
		RETURNING id, token, mcp_server_id, code_verifier_ciphertext, server_url, token_endpoint, resource, client_id,
		          client_secret_ciphertext, token_endpoint_auth_method, expires_at`, stateHash, at).
		Scan(&authorization.ID, &authorization.Token, &authorization.ServerID, &authorization.CodeVerifierCiphertext, &setup.ServerURL,
			&setup.TokenEndpoint, &setup.Resource, &setup.ClientID, &setup.ClientSecretCiphertext, &authMethod, &authorization.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return OAuthAuthorization{}, ErrOAuthStateInvalid
	}
	if err != nil {
		return OAuthAuthorization{}, fmt.Errorf("consume oauth authorization: %w", err)
	}
	setup.TokenEndpointAuthMethod = stringValue(authMethod)
	return authorization, nil
}

// InvalidatePendingAuthorizations ends every round trip still open for the server, so none completes against a changed URL.
func (McpServerOAuthRepository) InvalidatePendingAuthorizations(ctx context.Context, db platform.DBTX, serverID int64, at time.Time) error {
	_, err := db.Exec(ctx, `UPDATE mcp_server_oauth_authorizations SET consumed_at = $2 WHERE mcp_server_id = $1 AND consumed_at IS NULL`, serverID, at)
	if err != nil {
		return fmt.Errorf("invalidate oauth authorizations: %w", err)
	}
	return nil
}

func scanConnection(row pgx.Row) (OAuthConnection, error) {
	var connection OAuthConnection
	var serverURL, tokenEndpoint, resource, clientID, authMethod *string
	err := row.Scan(&connection.ID, &connection.ServerID, &connection.Status, &serverURL, &tokenEndpoint, &resource, &clientID,
		&connection.Setup.ClientSecretCiphertext, &authMethod, &connection.AccessTokenCiphertext, &connection.RefreshTokenCiphertext,
		&connection.ExpiresAt, &connection.ConnectedAt, &connection.LastError, &connection.CreatedAt, &connection.UpdatedAt)
	connection.Setup.ServerURL, connection.Setup.TokenEndpoint, connection.Setup.Resource = stringValue(serverURL), stringValue(tokenEndpoint), stringValue(resource)
	connection.Setup.ClientID, connection.Setup.TokenEndpointAuthMethod = stringValue(clientID), stringValue(authMethod)
	return connection, err
}

// DeleteFinishedAuthorizationsBefore removes round trips that were used or expired before the cutoff.
func (McpServerOAuthRepository) DeleteFinishedAuthorizationsBefore(ctx context.Context, db platform.DBTX, cutoff time.Time) (int, error) {
	tag, err := db.Exec(ctx, `
		DELETE FROM mcp_server_oauth_authorizations WHERE COALESCE(consumed_at, expires_at) < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete finished oauth authorizations: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
