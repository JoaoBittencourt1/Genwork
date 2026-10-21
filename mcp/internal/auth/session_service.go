package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform"
)

// RefreshMargin renews the access token shortly before it expires, so a request never leaves with a token about to die.
const RefreshMargin = 30 * time.Second

type SessionServiceConfig struct {
	InstanceURL string
	Client      *SessionClient
	Store       CredentialStore
	Lock        RefreshLock
	Now         func() time.Time
}

// SessionService signs in, keeps the access token in memory, and rotates the stored refresh token.
type SessionService struct {
	config               SessionServiceConfig
	mutex                sync.Mutex
	accessToken          string
	accessTokenExpiresAt time.Time
}

func NewSessionService(config SessionServiceConfig) *SessionService {
	if config.Now == nil {
		config.Now = time.Now
	}
	return &SessionService{config: config}
}

func (service *SessionService) Login(ctx context.Context, email, password string) error {
	email = strings.TrimSpace(email)
	if email == "" || password == "" {
		return ErrMissingCredentials
	}
	signIn, err := service.config.Client.Login(ctx, email, password)
	if err != nil {
		return err
	}
	credential := StoredCredential{Email: email, Session: signIn.Session, RefreshToken: signIn.RefreshToken}
	if err := service.config.Store.Save(service.config.InstanceURL, credential); err != nil {
		return err
	}
	service.keepAccessToken(signIn)
	return nil
}

// Logout revokes the session in core when it is still alive, and always forgets the local credential.
func (service *SessionService) Logout(ctx context.Context) error {
	if _, err := service.config.Store.Load(service.config.InstanceURL); err != nil {
		return err
	}
	accessToken, err := service.AccessToken(ctx)
	if err == nil {
		err = service.config.Client.Logout(ctx, accessToken)
	}
	if err != nil && !errors.Is(err, ErrSessionEnded) && !platform.IsAPIStatus(err, http.StatusUnauthorized) {
		return err
	}
	return service.config.Store.Delete(service.config.InstanceURL)
}

func (service *SessionService) SignedInEmail() (string, error) {
	credential, err := service.config.Store.Load(service.config.InstanceURL)
	return credential.Email, err
}

func (service *SessionService) AccessToken(ctx context.Context) (string, error) {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	if service.accessToken != "" && service.config.Now().Add(RefreshMargin).Before(service.accessTokenExpiresAt) {
		return service.accessToken, nil
	}
	signIn, err := service.refreshStoredSession(ctx)
	if err != nil {
		return "", err
	}
	service.accessToken = signIn.AccessToken
	service.accessTokenExpiresAt = signIn.AccessTokenExpiresAt
	return service.accessToken, nil
}

func (service *SessionService) Invalidate(rejectedAccessToken string) {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	if service.accessToken == rejectedAccessToken {
		service.accessToken = ""
	}
}

// refreshStoredSession reads the credential again under the lock: another process may have rotated it,
// and presenting an already-used refresh token makes core revoke the whole session.
func (service *SessionService) refreshStoredSession(ctx context.Context) (SignInResponse, error) {
	release, err := service.config.Lock.Acquire(ctx)
	if err != nil {
		return SignInResponse{}, err
	}
	defer release()
	credential, err := service.config.Store.Load(service.config.InstanceURL)
	if err != nil {
		return SignInResponse{}, err
	}
	signIn, err := service.config.Client.Refresh(ctx, credential.RefreshToken)
	if platform.IsAPIStatus(err, http.StatusUnauthorized) {
		return SignInResponse{}, ErrSessionEnded
	}
	if err != nil {
		return SignInResponse{}, err
	}
	credential.Session = signIn.Session
	credential.RefreshToken = signIn.RefreshToken
	if err := service.config.Store.Save(service.config.InstanceURL, credential); err != nil {
		return SignInResponse{}, err
	}
	return signIn, nil
}

func (service *SessionService) keepAccessToken(signIn SignInResponse) {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	service.accessToken = signIn.AccessToken
	service.accessTokenExpiresAt = signIn.AccessTokenExpiresAt
}
