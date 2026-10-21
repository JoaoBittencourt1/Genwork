package invites

import (
	"testing"
	"time"
)

func TestStatusIsDerivedFromTimestamps(t *testing.T) {
	createdAt := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	stamp := createdAt.Add(time.Hour)
	base := Invite{CreatedAt: createdAt, ExpiresAt: createdAt.Add(Lifetime)}

	accepted := base
	accepted.AcceptedAt = &stamp
	revoked := base
	revoked.RevokedAt = &stamp

	cases := []struct {
		name     string
		invite   Invite
		now      time.Time
		expected Status
	}{
		{"pending", base, createdAt.Add(time.Hour), StatusPending},
		{"expired one minute after seven days", base, createdAt.Add(Lifetime + time.Minute), StatusExpired},
		{"accepted wins over expiry", accepted, createdAt.Add(Lifetime + time.Minute), StatusAccepted},
		{"revoked", revoked, createdAt.Add(2 * time.Hour), StatusRevoked},
	}
	for _, testCase := range cases {
		if got := testCase.invite.StatusAt(testCase.now); got != testCase.expected {
			t.Errorf("%s: status = %s, want %s", testCase.name, got, testCase.expected)
		}
	}
}
