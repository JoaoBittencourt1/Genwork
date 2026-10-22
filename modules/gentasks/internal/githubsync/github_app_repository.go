package githubsync

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
	"github.com/jackc/pgx/v5"
)

type GithubAppRepository struct{}

// NewGithubApp.Token is chosen by the caller because it is part of the ciphertexts' associated data.
type NewGithubApp struct {
	Token                   string
	GithubAppID             int64
	Slug                    string
	OwnerLogin              string
	AppURL                  string
	WebhookSecretCiphertext []byte
	PrivateKeyCiphertext    []byte
	ConnectedBy             string
	At                      time.Time
}

const githubAppColumns = `id, token, github_app_id, slug, owner_login, app_url, webhook_secret_ciphertext, private_key_ciphertext,
	connected_by, created_at, updated_at`

func (GithubAppRepository) Create(ctx context.Context, db platform.DBTX, newApp NewGithubApp) (GithubApp, error) {
	app, err := scanGithubApp(db.QueryRow(ctx, `
		INSERT INTO github_apps (token, github_app_id, slug, owner_login, app_url, webhook_secret_ciphertext, private_key_ciphertext,
		                         connected_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
		RETURNING `+githubAppColumns,
		newApp.Token, newApp.GithubAppID, newApp.Slug, newApp.OwnerLogin, newApp.AppURL, newApp.WebhookSecretCiphertext,
		newApp.PrivateKeyCiphertext, newApp.ConnectedBy, newApp.At))
	if platform.IsUniqueViolation(err) {
		return GithubApp{}, ErrGithubAlreadyConnected
	}
	if err != nil {
		return GithubApp{}, fmt.Errorf("create github app: %w", err)
	}
	return app, nil
}

func (GithubAppRepository) Find(ctx context.Context, db platform.DBTX) (GithubApp, error) {
	app, err := scanGithubApp(db.QueryRow(ctx, `SELECT `+githubAppColumns+` FROM github_apps`))
	if errors.Is(err, pgx.ErrNoRows) {
		return GithubApp{}, ErrGithubNotConnected
	}
	if err != nil {
		return GithubApp{}, fmt.Errorf("find github app: %w", err)
	}
	return app, nil
}

func (GithubAppRepository) Delete(ctx context.Context, db platform.DBTX) error {
	if _, err := db.Exec(ctx, `DELETE FROM github_apps`); err != nil {
		return fmt.Errorf("delete github app: %w", err)
	}
	return nil
}

func scanGithubApp(row pgx.Row) (GithubApp, error) {
	var app GithubApp
	err := row.Scan(&app.ID, &app.Token, &app.GithubAppID, &app.Slug, &app.OwnerLogin, &app.AppURL, &app.WebhookSecretCiphertext,
		&app.PrivateKeyCiphertext, &app.ConnectedBy, &app.CreatedAt, &app.UpdatedAt)
	return app, err
}
