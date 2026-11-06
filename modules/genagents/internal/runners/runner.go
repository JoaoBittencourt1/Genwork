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
	ErrInvalidRegistrationToken = httpapi.NewAPIError(http.StatusUnauthorized, "invalid_registration_token")
	ErrInvalidRunnerSecret      = httpapi.NewAPIError(http.StatusUnauthorized, "invalid_runner_secret")
)

// Runner is a machine the admin controls that executes runs. It is revoked, never hard-deleted while runs reference it.
type Runner struct {
	ID                   int64
	Token                string
	Name                 string
	HasSecret            bool
	LastHeartbeatAt      *time.Time
	ContainerEngine      *string
	SandboxImage         *string
	CustomImage          bool
	DirectoryMode        bool
	ReportedTools        map[string]string
	HasGithubCredentials bool
	MaxConcurrency       int
	RevokedAt            *time.Time
	CreatedAt            time.Time
}

func (runner Runner) IsRevoked() bool {
	return runner.RevokedAt != nil
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
	At                   time.Time
}
