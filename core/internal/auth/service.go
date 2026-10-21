package auth

import (
	"context"
	"errors"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/core/internal/sessions"
	"github.com/JoaoBittencourt1/Genwork/core/internal/users"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	LockoutThreshold    = 10
	LockoutDuration     = 15 * time.Minute
	MaxDeviceNameLength = 100
)

var ErrInvalidCredentials = platform.NewAPIError(http.StatusUnauthorized, "invalid_credentials")

type SignIn struct {
	AccessToken          string
	AccessTokenExpiresAt time.Time
	RefreshToken         string
	Session              sessions.Session
}

type RolePermissionFinder interface {
	FindRolePermissions(ctx context.Context, roleID int64) ([]string, error)
}

type Service struct {
	db       *pgxpool.Pool
	hasher   *users.PasswordHasher
	issuer   *AccessTokenIssuer
	sessions *sessions.Service
	roles    RolePermissionFinder
	users    users.Repository
	now      func() time.Time
}

func NewService(db *pgxpool.Pool, hasher *users.PasswordHasher, issuer *AccessTokenIssuer, sessionService *sessions.Service, roles RolePermissionFinder, now func() time.Time) *Service {
	return &Service{db: db, hasher: hasher, issuer: issuer, sessions: sessionService, roles: roles, now: now}
}

type ClientInput struct {
	Client     sessions.ClientType
	DeviceName *string
}

func ValidateClientInput(input ClientInput) map[string]string {
	fields := map[string]string{}
	if !input.Client.IsValid() {
		fields["client"] = "invalid"
	}
	if input.DeviceName != nil && utf8.RuneCountInString(*input.DeviceName) > MaxDeviceNameLength {
		fields["deviceName"] = "too_long"
	}
	return fields
}

type LoginInput struct {
	Email    string
	Password string
	ClientInput
}

func (service *Service) Login(ctx context.Context, input LoginInput) (SignIn, error) {
	fields := ValidateClientInput(input.ClientInput)
	email := users.NormalizeEmail(input.Email)
	if !users.IsValidEmail(email) {
		fields["email"] = "invalid"
	}
	if len(fields) > 0 {
		return SignIn{}, platform.NewValidationError(fields)
	}

	user, err := service.users.FindByEmail(ctx, service.db, email)
	if errors.Is(err, users.ErrUserNotFound) {
		service.hasher.VerifyAgainstDummy(input.Password)
		return SignIn{}, ErrInvalidCredentials
	}
	if err != nil {
		return SignIn{}, err
	}

	now := service.now()
	matches, needsRehash, err := service.hasher.Verify(input.Password, user.PasswordHash)
	if err != nil {
		return SignIn{}, err
	}
	if user.IsLockedAt(now) {
		return SignIn{}, ErrInvalidCredentials
	}
	if !matches {
		if err := service.users.RecordFailedLogin(ctx, service.db, user.ID, LockoutThreshold, now.Add(LockoutDuration)); err != nil {
			return SignIn{}, err
		}
		return SignIn{}, ErrInvalidCredentials
	}

	var signIn SignIn
	err = pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		if err := service.users.RecordSuccessfulLogin(ctx, tx, user.ID, now); err != nil {
			return err
		}
		if needsRehash {
			if err := service.users.UpdatePasswordHash(ctx, tx, user.ID, service.hasher.Hash(input.Password)); err != nil {
				return err
			}
		}
		signIn, err = service.StartSignIn(ctx, tx, user.ID, input.ClientInput)
		return err
	})
	return signIn, err
}

// StartSignIn runs on the caller's connection or transaction so flows like invite acceptance can sign the user in atomically.
func (service *Service) StartSignIn(ctx context.Context, db platform.DBTX, userID int64, client ClientInput) (SignIn, error) {
	session, refreshToken, err := service.sessions.Start(ctx, db, sessions.StartInput{
		UserID: userID, ClientType: client.Client, DeviceName: client.DeviceName,
	})
	if err != nil {
		return SignIn{}, err
	}
	return service.issueAccessToken(ctx, session, refreshToken)
}

func (service *Service) Refresh(ctx context.Context, refreshToken string) (SignIn, error) {
	session, rotatedRefreshToken, err := service.sessions.Rotate(ctx, refreshToken)
	if err != nil {
		return SignIn{}, err
	}
	return service.issueAccessToken(ctx, session, rotatedRefreshToken)
}

func (service *Service) issueAccessToken(ctx context.Context, session sessions.Session, refreshToken string) (SignIn, error) {
	permissions, err := service.roles.FindRolePermissions(ctx, session.RoleID)
	if err != nil {
		return SignIn{}, err
	}
	accessToken, expiresAt, err := service.issuer.Issue(session.UserToken, session.Token, permissions)
	if err != nil {
		return SignIn{}, err
	}
	return SignIn{
		AccessToken:          accessToken,
		AccessTokenExpiresAt: expiresAt,
		RefreshToken:         refreshToken,
		Session:              session,
	}, nil
}
