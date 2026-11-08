package mcpservers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const secretsColumn = "mcp_servers.secrets"

type McpServerService struct {
	db      *pgxpool.Pool
	cipher  *platform.SecretCipher
	servers McpServerRepository
	oauth   *McpServerOAuthService
	conns   McpServerOAuthRepository
	now     func() time.Time
}

func NewMcpServerService(db *pgxpool.Pool, cipher *platform.SecretCipher, now func() time.Time) *McpServerService {
	return &McpServerService{db: db, cipher: cipher, now: now}
}

// WithOAuth makes runs receive the bearer token of OAuth-connected servers.
func (service *McpServerService) WithOAuth(oauth *McpServerOAuthService) *McpServerService {
	service.oauth = oauth
	return service
}

func associatedData(token string) string {
	return secretsColumn + "|" + token
}

func (service *McpServerService) ListServers(ctx context.Context) ([]McpServer, error) {
	stored, err := service.servers.List(ctx, service.db)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(stored))
	for _, server := range stored {
		ids = append(ids, server.ID)
	}
	connections, err := service.conns.ListByServers(ctx, service.db, ids)
	if err != nil {
		return nil, err
	}
	listed := make([]McpServer, 0, len(stored))
	for _, server := range stored {
		if connection, found := connections[server.ID]; found {
			server.OAuth = &connection
		}
		listed = append(listed, server.McpServer)
	}
	return listed, nil
}

func (service *McpServerService) FindServer(ctx context.Context, token string) (McpServer, error) {
	stored, err := service.servers.FindByToken(ctx, service.db, token)
	return stored.McpServer, err
}

func (service *McpServerService) CreateServer(ctx context.Context, server McpServer) (McpServer, error) {
	token := platform.NewPublicToken()
	config, secrets := splitSecrets(server)
	ciphertext, err := service.encryptSecrets(token, secrets)
	if err != nil {
		return McpServer{}, err
	}
	stored, err := service.servers.Create(ctx, service.db, token, server.Name, server.Transport, config, ciphertext, service.now())
	return stored.McpServer, err
}

func (service *McpServerService) UpdateServer(ctx context.Context, token string, request UpdateMcpServerRequest) (McpServer, error) {
	var updated StoredMcpServer
	err := pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		stored, err := service.servers.FindByToken(ctx, tx, token)
		if err != nil {
			return err
		}
		existing, err := service.decryptSecrets(stored)
		if err != nil {
			return err
		}
		changed, err := request.ApplyTo(stored.McpServer, existing)
		if err != nil {
			return err
		}
		config, secrets := splitSecrets(changed)
		ciphertext, err := service.encryptSecrets(token, secrets)
		if err != nil {
			return err
		}
		if err := service.servers.Update(ctx, tx, stored.ID, changed.Name, config, ciphertext, service.now()); err != nil {
			return err
		}
		updated, err = service.servers.FindByToken(ctx, tx, token)
		return err
	})
	return updated.McpServer, err
}

func (service *McpServerService) DeleteServer(ctx context.Context, token string) error {
	return pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		stored, err := service.servers.FindByToken(ctx, tx, token)
		if err != nil {
			return err
		}
		return service.servers.SoftDelete(ctx, tx, stored.ID, service.now())
	})
}

// ResolveForRun returns the servers with their secrets decrypted and the bearer token of OAuth-connected ones; only run
// claiming calls it, and only to hand the values to a runner once.
func (service *McpServerService) ResolveForRun(ctx context.Context, db platform.DBTX, ids []int64) ([]McpServer, error) {
	stored, err := service.servers.ListByIDs(ctx, db, ids)
	if err != nil {
		return nil, err
	}
	for index := range stored {
		secrets, err := service.decryptSecrets(stored[index])
		if err != nil {
			return nil, err
		}
		stored[index].Secrets = secrets
	}
	if service.oauth != nil {
		if err := service.oauth.AttachTokens(ctx, db, stored); err != nil {
			return nil, err
		}
	}
	resolved := make([]McpServer, 0, len(stored))
	for _, server := range stored {
		resolved = append(resolved, server.McpServer)
	}
	return resolved, nil
}

// splitSecrets moves every secret value out of the clear config into the secrets map.
func splitSecrets(server McpServer) (Config, map[string]string) {
	secrets := map[string]string{}
	config := server.Config
	config.Headers = scrub("header", server.Config.Headers, server.Secrets, secrets)
	config.Env = scrub("env", server.Config.Env, server.Secrets, secrets)
	return config, secrets
}

func scrub(kind string, entries []Entry, known, secrets map[string]string) []Entry {
	scrubbed := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if entry.Secret {
			value := entry.Value
			if value == "" {
				value = known[SecretKey(kind, entry.Name)]
			}
			secrets[SecretKey(kind, entry.Name)] = value
			entry.Value = ""
		}
		entry.HasValue = false
		scrubbed = append(scrubbed, entry)
	}
	return scrubbed
}

func (service *McpServerService) encryptSecrets(token string, secrets map[string]string) ([]byte, error) {
	if len(secrets) == 0 {
		return nil, nil
	}
	encoded, err := json.Marshal(secrets)
	if err != nil {
		return nil, fmt.Errorf("encode mcp server secrets: %w", err)
	}
	return service.cipher.Encrypt(string(encoded), associatedData(token))
}

func (service *McpServerService) decryptSecrets(stored StoredMcpServer) (map[string]string, error) {
	secrets := map[string]string{}
	if len(stored.SecretsCiphertext) == 0 {
		return secrets, nil
	}
	plaintext, err := service.cipher.Decrypt(stored.SecretsCiphertext, associatedData(stored.Token))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(plaintext), &secrets); err != nil {
		return nil, fmt.Errorf("decode mcp server secrets: %w", err)
	}
	return secrets, nil
}
