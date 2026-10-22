package githubsync

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrGithubUnavailableResponse = httpapi.NewAPIError(http.StatusBadGateway, "github_unavailable")

// ManifestConverter is the boundary to GitHub's manifest conversion endpoint (rule 9).
type ManifestConverter interface {
	ConvertManifest(ctx context.Context, code string) (ConvertedGithubApp, error)
}

type GithubConnectionConfig struct {
	PublicURL    string
	GithubWebURL string
}

// GithubConnection.App and InstallURL are set whenever an App is stored, even when its credentials are unreadable.
type GithubConnection struct {
	State      ConnectionState
	WebhookURL string
	App        *GithubApp
	InstallURL string
}

type ManifestStart struct {
	FormAction string
	Manifest   AppManifest
	State      string
}

type GithubConnectionService struct {
	db        *pgxpool.Pool
	apps      GithubAppRepository
	cipher    *CredentialCipher
	states    *ManifestStateSigner
	converter ManifestConverter
	config    GithubConnectionConfig
	now       func() time.Time
}

func NewGithubConnectionService(db *pgxpool.Pool, cipher *CredentialCipher, states *ManifestStateSigner, converter ManifestConverter,
	config GithubConnectionConfig, now func() time.Time) *GithubConnectionService {
	return &GithubConnectionService{db: db, cipher: cipher, states: states, converter: converter, config: config, now: now}
}

func (service *GithubConnectionService) FindConnection(ctx context.Context) (GithubConnection, error) {
	app, err := service.apps.Find(ctx, service.db)
	if errors.Is(err, ErrGithubNotConnected) {
		return GithubConnection{State: ConnectionDisconnected, WebhookURL: WebhookURL(service.config.PublicURL)}, nil
	}
	if err != nil {
		return GithubConnection{}, err
	}
	state := ConnectionConnected
	if _, err := service.decryptWebhookSecret(app); err != nil {
		state = ConnectionUnreadable
	} else if _, err := service.cipher.Decrypt(app.PrivateKeyCiphertext, PrivateKeyAssociatedData(app.Token)); err != nil {
		state = ConnectionUnreadable
	}
	return service.describeStoredApp(app, state), nil
}

func (service *GithubConnectionService) StartManifest(ctx context.Context, userToken string, input StartManifestInput) (ManifestStart, error) {
	if err := service.ensureDisconnected(ctx); err != nil {
		return ManifestStart{}, err
	}
	state, err := service.states.Issue(userToken, service.now())
	if err != nil {
		return ManifestStart{}, err
	}
	return ManifestStart{
		FormAction: ManifestFormAction(service.config.GithubWebURL, input.Organization, state),
		Manifest:   NewAppManifest(service.config.PublicURL),
		State:      state,
	}, nil
}

func (service *GithubConnectionService) CompleteConnection(ctx context.Context, userToken string, input CompleteConnectionInput) (GithubConnection, error) {
	if err := service.states.Verify(input.State, userToken, service.now()); err != nil {
		return GithubConnection{}, httpapi.NewValidationError(map[string]string{"state": "invalid"})
	}
	if err := service.ensureDisconnected(ctx); err != nil {
		return GithubConnection{}, err
	}
	converted, err := service.converter.ConvertManifest(ctx, input.Code)
	if errors.Is(err, ErrManifestCodeRejected) {
		return GithubConnection{}, httpapi.NewValidationError(map[string]string{"code": "invalid"})
	}
	if errors.Is(err, ErrGithubUnavailable) {
		slog.WarnContext(ctx, "github manifest conversion failed", "err", err)
		return GithubConnection{}, ErrGithubUnavailableResponse
	}
	if err != nil {
		return GithubConnection{}, err
	}
	app, err := service.storeConvertedApp(ctx, userToken, converted)
	if err != nil {
		return GithubConnection{}, err
	}
	return service.describeStoredApp(app, ConnectionConnected), nil
}

// VerifyDelivery rejects a webhook delivery unless the stored App's secret signed it; with no readable App, nothing verifies.
func (service *GithubConnectionService) VerifyDelivery(ctx context.Context, body []byte, signatureHeader string) error {
	app, err := service.apps.Find(ctx, service.db)
	if errors.Is(err, ErrGithubNotConnected) {
		return ErrInvalidWebhookSignature
	}
	if err != nil {
		return err
	}
	secret, err := service.decryptWebhookSecret(app)
	if err != nil {
		return ErrInvalidWebhookSignature
	}
	if !VerifyWebhookSignature(secret, body, signatureHeader) {
		return ErrInvalidWebhookSignature
	}
	return nil
}

func (service *GithubConnectionService) Disconnect(ctx context.Context) error {
	return service.apps.Delete(ctx, service.db)
}

func (service *GithubConnectionService) storeConvertedApp(ctx context.Context, userToken string, converted ConvertedGithubApp) (GithubApp, error) {
	token := platform.NewPublicToken()
	webhookSecret, err := service.cipher.Encrypt(converted.WebhookSecret, WebhookSecretAssociatedData(token))
	if err != nil {
		return GithubApp{}, err
	}
	privateKey, err := service.cipher.Encrypt(converted.PrivateKey, PrivateKeyAssociatedData(token))
	if err != nil {
		return GithubApp{}, err
	}
	return service.apps.Create(ctx, service.db, NewGithubApp{
		Token: token, GithubAppID: converted.GithubAppID, Slug: converted.Slug, OwnerLogin: converted.OwnerLogin,
		AppURL:                  service.appSettingsURL(converted),
		WebhookSecretCiphertext: webhookSecret, PrivateKeyCiphertext: privateKey, ConnectedBy: userToken, At: service.now(),
	})
}

func (service *GithubConnectionService) ensureDisconnected(ctx context.Context) error {
	_, err := service.apps.Find(ctx, service.db)
	if errors.Is(err, ErrGithubNotConnected) {
		return nil
	}
	if err != nil {
		return err
	}
	return ErrGithubAlreadyConnected
}

func (service *GithubConnectionService) decryptWebhookSecret(app GithubApp) (GithubCredential, error) {
	return service.cipher.Decrypt(app.WebhookSecretCiphertext, WebhookSecretAssociatedData(app.Token))
}

func (service *GithubConnectionService) describeStoredApp(app GithubApp, state ConnectionState) GithubConnection {
	return GithubConnection{
		State:      state,
		WebhookURL: WebhookURL(service.config.PublicURL),
		App:        &app,
		InstallURL: service.config.GithubWebURL + "/apps/" + url.PathEscape(app.Slug) + "/installations/new",
	}
}

// appSettingsURL is where the owner manages or deletes the App; GitHub serves it under the owning account.
func (service *GithubConnectionService) appSettingsURL(converted ConvertedGithubApp) string {
	slug := url.PathEscape(converted.Slug)
	if converted.OwnerIsOrganization {
		return service.config.GithubWebURL + "/organizations/" + url.PathEscape(converted.OwnerLogin) + "/settings/apps/" + slug
	}
	return service.config.GithubWebURL + "/settings/apps/" + slug
}
