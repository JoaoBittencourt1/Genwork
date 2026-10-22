package oauth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/jackc/pgx/v5"
)

var errClientNotFound = errors.New("oauth client not found")

type OAuthClientRepository struct{}

type NewOAuthClient struct {
	Name         string
	RedirectURIs []string
	Now          time.Time
}

const oauthClientColumns = `id, token, name, redirect_uris, created_at`

func (OAuthClientRepository) Create(ctx context.Context, db platform.DBTX, newClient NewOAuthClient) (OAuthClient, error) {
	client, err := scanOAuthClient(db.QueryRow(ctx, `
		INSERT INTO oauth_clients (name, redirect_uris, created_at) VALUES ($1, $2, $3)
		RETURNING `+oauthClientColumns, newClient.Name, newClient.RedirectURIs, newClient.Now))
	if err != nil {
		return OAuthClient{}, fmt.Errorf("create oauth client: %w", err)
	}
	return client, nil
}

func (OAuthClientRepository) FindByToken(ctx context.Context, db platform.DBTX, token string) (OAuthClient, error) {
	client, err := scanOAuthClient(db.QueryRow(ctx, `
		SELECT `+oauthClientColumns+` FROM oauth_clients WHERE token = $1 AND deleted_at IS NULL`, token))
	if errors.Is(err, pgx.ErrNoRows) {
		return OAuthClient{}, errClientNotFound
	}
	if err != nil {
		return OAuthClient{}, fmt.Errorf("find oauth client: %w", err)
	}
	return client, nil
}

func scanOAuthClient(row pgx.Row) (OAuthClient, error) {
	var client OAuthClient
	err := row.Scan(&client.ID, &client.Token, &client.Name, &client.RedirectURIs, &client.CreatedAt)
	return client, err
}
