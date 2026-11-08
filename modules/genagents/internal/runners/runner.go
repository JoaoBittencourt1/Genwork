package runners

import (
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

const (
	RegistrationLifetime = time.Hour
	OnlineThreshold      = 90 * time.Second
	LostThreshold        = 5 * time.Minute
	DefaultConcurrency   = 2
	MaxRunnerNameLength  = 80
)

var (
	ErrRunnerNotFound           = httpapi.NewAPIError(http.StatusNotFound, "runner_not_found")
	ErrRunnerNameTaken          = httpapi.NewAPIError(http.StatusConflict, "runner_name_taken")
	ErrRunnerNotRemovable       = httpapi.NewAPIError(http.StatusConflict, "runner_not_removable")
	ErrInvalidRegistrationToken = httpapi.NewAPIError(http.StatusUnauthorized, "invalid_registration_token")
	ErrInvalidRunnerSecret      = httpapi.NewAPIError(http.StatusUnauthorized, "invalid_runner_secret")
)

// Runner is a machine the admin controls that executes runs. Removal is a soft delete (rule 22): runs keep their
// reference to the row, and the name can be used again.
type Runner struct {
	ID                    int64
	Token                 string
	Name                  string
	HasSecret             bool
	LastHeartbeatAt       *time.Time
	ContainerEngine       *string
	SandboxImage          *string
	CustomImage           bool
	DirectoryMode         bool
	ReportedTools         map[string]string
	HasGithubCredentials  bool
	MaxConcurrency        int
	RevokedAt             *time.Time
	CreatedAt             time.Time
	RegistrationExpiresAt *time.Time
	CleanupRequestedAt    *time.Time
	LastCleanupAt         *time.Time
	LastCleanupFreedBytes *int64
}

// CleanupPending says an admin asked the runner to free space and it has not reported doing it since.
func (runner Runner) CleanupPending() bool {
	return runner.CleanupRequestedAt != nil && (runner.LastCleanupAt == nil || runner.LastCleanupAt.Before(*runner.CleanupRequestedAt))
}

func (runner Runner) IsRevoked() bool {
	return runner.RevokedAt != nil
}

// CanBeRemoved reports whether an admin may drop the runner from the list. A revoked runner is already rejected.
// One that never registered can be removed once its one-hour token has expired; until then, revoke is how it is cancelled.
func (runner Runner) CanBeRemoved(now time.Time) bool {
	if runner.IsRevoked() {
		return true
	}
	if runner.HasSecret || runner.RegistrationExpiresAt == nil {
		return false
	}
	return !runner.RegistrationExpiresAt.After(now)
}

// IsOnline derives presence from the last heartbeat, so no stored status can disagree with it.
func (runner Runner) IsOnline(now time.Time) bool {
	return !runner.IsRevoked() && runner.LastHeartbeatAt != nil && now.Sub(*runner.LastHeartbeatAt) < OnlineThreshold
}

func (runner Runner) IsLost(now time.Time) bool {
	return runner.IsRevoked() || runner.LastHeartbeatAt == nil || now.Sub(*runner.LastHeartbeatAt) >= LostThreshold
}

func (runner Runner) ReportsTool(cli string) bool {
	_, found := runner.ReportedTools[cli]
	return found
}

// Heartbeat is what a runner reports about itself every few seconds.
type Heartbeat struct {
	ContainerEngine      string
	SandboxImage         string
	CustomImage          bool
	DirectoryMode        bool
	Tools                map[string]string
	HasGithubCredentials bool
	MaxConcurrency       int
	// CleanupFreedBytes is set on the one heartbeat after the runner finished a requested cleanup.
	CleanupFreedBytes *int64
	At                time.Time
}
