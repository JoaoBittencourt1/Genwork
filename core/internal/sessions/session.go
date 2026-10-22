package sessions

import (
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
)

type ClientType string

const (
	ClientWeb    ClientType = "web"
	ClientNative ClientType = "native"
	// ClientOAuth is an AI assistant's grant; it starts only through the OAuth code exchange, never a password sign-in.
	ClientOAuth ClientType = "oauth"
)

// SignsInWithPassword tells the client types a password sign-in may start.
func (client ClientType) SignsInWithPassword() bool {
	return client == ClientWeb || client == ClientNative
}

const (
	IdleLifetime     = 30 * 24 * time.Hour
	AbsoluteLifetime = 90 * 24 * time.Hour
)

var (
	ErrSessionRevoked = platform.NewAPIError(http.StatusUnauthorized, "session_revoked")
	ErrSessionExpired = platform.NewAPIError(http.StatusUnauthorized, "session_expired")
)

type Session struct {
	ID                int64
	Token             string
	UserID            int64
	UserToken         string
	RoleID            int64
	ClientType        ClientType
	DeviceName        *string
	OAuthClientID     *int64
	CreatedAt         time.Time
	LastUsedAt        time.Time
	IdleExpiresAt     time.Time
	AbsoluteExpiresAt time.Time
	RevokedAt         *time.Time
}

func (session Session) IsExpiredAt(now time.Time) bool {
	return !now.Before(session.IdleExpiresAt) || !now.Before(session.AbsoluteExpiresAt)
}

func nextIdleExpiry(now, absoluteExpiresAt time.Time) time.Time {
	idleExpiresAt := now.Add(IdleLifetime)
	if idleExpiresAt.After(absoluteExpiresAt) {
		return absoluteExpiresAt
	}
	return idleExpiresAt
}
