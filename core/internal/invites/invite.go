package invites

import (
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
)

const Lifetime = 7 * 24 * time.Hour

type Status string

const (
	StatusPending  Status = "pending"
	StatusAccepted Status = "accepted"
	StatusRevoked  Status = "revoked"
	StatusExpired  Status = "expired"
)

func ParseStatus(raw string) (Status, bool) {
	status := Status(raw)
	switch status {
	case StatusPending, StatusAccepted, StatusRevoked, StatusExpired:
		return status, true
	}
	return "", false
}

var (
	ErrInviteNotFound       = platform.NewAPIError(http.StatusNotFound, "invite_not_found")
	ErrInviteAlreadyPending = platform.NewAPIError(http.StatusConflict, "invite_already_pending")
	ErrInviteNotPending     = platform.NewAPIError(http.StatusConflict, "invite_not_pending")
)

type Invite struct {
	ID              int64
	Token           string
	Email           string
	RoleID          int64
	RoleToken       string
	RoleName        string
	InvitedByUserID int64
	InviterToken    string
	InviterName     string
	CreatedAt       time.Time
	ExpiresAt       time.Time
	AcceptedAt      *time.Time
	RevokedAt       *time.Time
}

// StatusAt derives the status from timestamps so there is no stored status that could disagree with them.
func (invite Invite) StatusAt(now time.Time) Status {
	switch {
	case invite.AcceptedAt != nil:
		return StatusAccepted
	case invite.RevokedAt != nil:
		return StatusRevoked
	case !now.Before(invite.ExpiresAt):
		return StatusExpired
	default:
		return StatusPending
	}
}
