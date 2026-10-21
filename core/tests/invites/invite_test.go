package invites_test

import (
	"github.com/JoaoBittencourt1/Genwork/core/internal/invites"
	"testing"
	"time"
)

func TestStatusIsDerivedFromTimestamps(t *testing.T) {
	createdAt := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	stamp := createdAt.Add(time.Hour)
	base := invites.Invite{CreatedAt: createdAt, ExpiresAt: createdAt.Add(invites.Lifetime)}

	accepted := base
	accepted.AcceptedAt = &stamp
	revoked := base
	revoked.RevokedAt = &stamp

	cases := []struct {
		name     string
		invite   invites.Invite
		now      time.Time
		expected invites.Status
	}{
		{"pending", base, createdAt.Add(time.Hour), invites.StatusPending},
		{"expired one minute after seven days", base, createdAt.Add(invites.Lifetime + time.Minute), invites.StatusExpired},
		{"accepted wins over expiry", accepted, createdAt.Add(invites.Lifetime + time.Minute), invites.StatusAccepted},
		{"revoked", revoked, createdAt.Add(2 * time.Hour), invites.StatusRevoked},
	}
	for _, testCase := range cases {
		if got := testCase.invite.StatusAt(testCase.now); got != testCase.expected {
			t.Errorf("%s: status = %s, want %s", testCase.name, got, testCase.expected)
		}
	}
}
