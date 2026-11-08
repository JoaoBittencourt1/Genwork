package githubsync

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/coreclient"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
	"github.com/jackc/pgx/v5/pgxpool"
)

// InstallationTokenIssuer is the boundary to GitHub's App endpoints (rule 9).
type InstallationTokenIssuer interface {
	FindRepositoryInstallation(ctx context.Context, appJWT, repository string) (int64, error)
	CreateInstallationToken(ctx context.Context, appJWT string, installationID int64, repositoryNames []string, permissions map[string]string) (InstallationToken, error)
}

// MemberDirectory tells what kind of member a caller is, from core's user directory (rule 9).
type MemberDirectory interface {
	FindUser(ctx context.Context, accessToken, userToken string) (coreclient.DirectoryUser, error)
}

// RepositoryTokenService issues short-lived installation tokens of the gentasks GitHub App for the repositories bots work
// in, so runners need no GitHub token of their own. A token is never stored.
type RepositoryTokenService struct {
	db      *pgxpool.Pool
	apps    GithubAppRepository
	scope   RepositoryScopeRepository
	cipher  *CredentialCipher
	github  InstallationTokenIssuer
	members MemberDirectory
	now     func() time.Time
}

func NewRepositoryTokenService(db *pgxpool.Pool, cipher *CredentialCipher, github InstallationTokenIssuer, members MemberDirectory, now func() time.Time) *RepositoryTokenService {
	return &RepositoryTokenService{db: db, cipher: cipher, github: github, members: members, now: now}
}

// IssueToken requires a bot member besides the permission, so the App's write access never becomes a person's way
// around their own GitHub rights.
func (service *RepositoryTokenService) IssueToken(ctx context.Context, caller TokenCaller, repositories []string) (InstallationToken, error) {
	if err := service.ensureBotMember(ctx, caller); err != nil {
		return InstallationToken{}, err
	}
	if err := service.ensureKnown(ctx, repositories); err != nil {
		return InstallationToken{}, err
	}
	appJWT, err := service.signAppJWT(ctx)
	if err != nil {
		return InstallationToken{}, err
	}
	installationID, err := service.findSharedInstallation(ctx, appJWT, repositories)
	if err != nil {
		return InstallationToken{}, err
	}
	names := make([]string, 0, len(repositories))
	for _, repository := range repositories {
		names = append(names, repositoryName(repository))
	}
	token, err := service.github.CreateInstallationToken(ctx, appJWT, installationID, names, RepositoryTokenPermissions)
	return token, translateGithubError(err)
}

func (service *RepositoryTokenService) ensureBotMember(ctx context.Context, caller TokenCaller) error {
	member, err := service.members.FindUser(ctx, caller.AccessToken, caller.UserToken)
	switch {
	case errors.Is(err, coreclient.ErrUserNotFound), errors.Is(err, coreclient.ErrDirectoryForbidden):
		return ErrRepositoryTokensForBotsOnly
	case err != nil:
		return platform.ErrCoreUnavailable
	case member.Kind != "bot":
		return ErrRepositoryTokensForBotsOnly
	}
	return nil
}

func (service *RepositoryTokenService) ensureKnown(ctx context.Context, repositories []string) error {
	known, err := service.scope.ListKnown(ctx, service.db, repositories)
	if err != nil {
		return err
	}
	for _, repository := range repositories {
		if !known[strings.ToLower(repository)] {
			return httpapi.NewValidationError(map[string]string{"repositories": "unknown"})
		}
	}
	return nil
}

func (service *RepositoryTokenService) signAppJWT(ctx context.Context) (string, error) {
	app, err := service.apps.Find(ctx, service.db)
	if err != nil {
		return "", err
	}
	privateKey, err := service.cipher.Decrypt(app.PrivateKeyCiphertext, PrivateKeyAssociatedData(app.Token))
	if err != nil {
		return "", ErrGithubAppCredentialsUnreadable
	}
	appJWT, err := SignAppJWT(privateKey, app.GithubAppID, service.now())
	if errors.Is(err, ErrPrivateKeyInvalid) {
		return "", ErrGithubAppCredentialsUnreadable
	}
	return appJWT, err
}

// findSharedInstallation requires one installation for every repository, since a token belongs to one installation.
func (service *RepositoryTokenService) findSharedInstallation(ctx context.Context, appJWT string, repositories []string) (int64, error) {
	var shared int64
	for _, repository := range repositories {
		installationID, err := service.github.FindRepositoryInstallation(ctx, appJWT, repository)
		if err != nil {
			return 0, translateGithubError(err)
		}
		if shared != 0 && installationID != shared {
			return 0, ErrRepositoriesAcrossInstallations
		}
		shared = installationID
	}
	return shared, nil
}

func translateGithubError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrAppNotInstalled):
		return ErrGithubAppNotInstalled
	case errors.Is(err, ErrAppPermissionsDenied):
		return ErrGithubAppPermissionsMissing
	case errors.Is(err, ErrGithubUnavailable):
		return ErrGithubUnavailableResponse
	}
	return err
}
