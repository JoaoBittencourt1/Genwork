package sessions

import (
	"context"
	"errors"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db       *pgxpool.Pool
	sessions Repository
	now      func() time.Time
}

func NewService(db *pgxpool.Pool, now func() time.Time) *Service {
	return &Service{db: db, now: now}
}

type StartInput struct {
	UserID     int64
	ClientType ClientType
	DeviceName *string
	// OAuthClientID is the OAuth client of an oauth session, and nil for every other client type.
	OAuthClientID *int64
}

// ErrRefreshTokenOfOtherClient means the refresh token belongs to a session the presenting client does not own.
var ErrRefreshTokenOfOtherClient = errors.New("refresh token of another client")

// Start runs on the caller's connection or transaction so a sign-in can be part of a larger unit of work.
func (service *Service) Start(ctx context.Context, db platform.DBTX, input StartInput) (Session, string, error) {
	now := service.now()
	session, err := service.sessions.Create(ctx, db, NewSession{
		UserID: input.UserID, ClientType: input.ClientType, DeviceName: input.DeviceName, OAuthClientID: input.OAuthClientID, Now: now,
	})
	if err != nil {
		return Session{}, "", err
	}
	refreshSecret, err := service.sessions.CreateRefreshToken(ctx, db, session.ID, now)
	if err != nil {
		return Session{}, "", err
	}
	return session, refreshSecret, nil
}

// Rotate exchanges the refresh token of a web or native session for a new one; an assistant's token rotates only
// through RotateForOAuthClient.
func (service *Service) Rotate(ctx context.Context, refreshSecret string) (Session, string, error) {
	return service.rotate(ctx, refreshSecret, func(session Session) error {
		if session.ClientType == ClientOAuth {
			return platform.ErrInvalidToken
		}
		return nil
	})
}

// RotateForOAuthClient rotates only a session that client obtained, and leaves any other session untouched.
func (service *Service) RotateForOAuthClient(ctx context.Context, refreshSecret string, oauthClientID int64) (Session, string, error) {
	return service.rotate(ctx, refreshSecret, func(session Session) error {
		if session.OAuthClientID == nil || *session.OAuthClientID != oauthClientID {
			return ErrRefreshTokenOfOtherClient
		}
		return nil
	})
}

// rotate refuses before changing anything when accept refuses the session. Presenting a token that was already used
// means it leaked, so the whole session is revoked and the revocation is committed even though the call fails.
func (service *Service) rotate(ctx context.Context, refreshSecret string, accept func(Session) error) (Session, string, error) {
	now := service.now()
	var rotated Session
	var newSecret string
	var outcome error
	err := pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		locked, err := service.sessions.LockRefreshToken(ctx, tx, refreshSecret)
		if errors.Is(err, ErrRefreshTokenNotFound) {
			outcome = platform.ErrInvalidToken
			return nil
		}
		if err != nil {
			return err
		}
		session := locked.Session
		if outcome = accept(session); outcome != nil {
			return nil
		}
		switch {
		case session.RevokedAt != nil:
			outcome = ErrSessionRevoked
			return nil
		case locked.UsedAt != nil:
			outcome = ErrSessionRevoked
			return service.sessions.Revoke(ctx, tx, session.ID, now)
		case session.IsExpiredAt(now):
			outcome = ErrSessionExpired
			return nil
		}
		if err := service.sessions.MarkRefreshTokenUsed(ctx, tx, locked.ID, now); err != nil {
			return err
		}
		if rotated, err = service.sessions.ExtendIdleExpiry(ctx, tx, session, now); err != nil {
			return err
		}
		newSecret, err = service.sessions.CreateRefreshToken(ctx, tx, session.ID, now)
		return err
	})
	if err != nil {
		return Session{}, "", err
	}
	if outcome != nil {
		return Session{}, "", outcome
	}
	return rotated, newSecret, nil
}

func (service *Service) FindActive(ctx context.Context, sessionToken string) (Session, error) {
	return service.sessions.FindActiveByToken(ctx, service.db, sessionToken, service.now())
}

func (service *Service) Revoke(ctx context.Context, sessionID int64) error {
	return service.sessions.Revoke(ctx, service.db, sessionID, service.now())
}

func (service *Service) ListActiveForUser(ctx context.Context, userID int64) ([]Session, error) {
	return service.sessions.ListActiveForUser(ctx, service.db, userID, service.now())
}

func (service *Service) RevokeForUser(ctx context.Context, userID int64, sessionToken string) error {
	err := service.sessions.RevokeForUser(ctx, service.db, userID, sessionToken, service.now())
	if errors.Is(err, ErrSessionNotFound) {
		return platform.ErrNotFound
	}
	return err
}
