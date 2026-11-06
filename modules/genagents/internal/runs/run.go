// Package runs holds runs, conversations, and their entries: what a bot did, said, and was told.
package runs

import (
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

type RunKind string

const (
	RunKindTask   RunKind = "task"
	RunKindReview RunKind = "review"
	RunKindChat   RunKind = "chat"
)

// NeedsGithub says whether a run clones and posts on GitHub, so only runners with GitHub credentials may claim it.
func (kind RunKind) NeedsGithub() bool {
	return kind == RunKindTask || kind == RunKindReview
}

type RunState string

const (
	RunStateQueued          RunState = "queued"
	RunStateRunning         RunState = "running"
	RunStateWaitingForInput RunState = "waiting_for_input"
	RunStateSucceeded       RunState = "succeeded"
	RunStateFailed          RunState = "failed"
	RunStateCancelled       RunState = "cancelled"
)

func (state RunState) IsActive() bool {
	return state == RunStateQueued || state == RunStateRunning || state == RunStateWaitingForInput
}

func (state RunState) IsValid() bool {
	switch state {
	case RunStateQueued, RunStateRunning, RunStateWaitingForInput, RunStateSucceeded, RunStateFailed, RunStateCancelled:
		return true
	}
	return false
}

type FailureReason string

const (
	FailureRunnerRevoked    FailureReason = "runner_revoked"
	FailureRunnerLost       FailureReason = "runner_lost"
	FailureTimeout          FailureReason = "timeout"
	FailureSandboxLimit     FailureReason = "sandbox_limit"
	FailureAdapterError     FailureReason = "adapter_error"
	FailureNoRepository     FailureReason = "no_repository"
	FailureNoReply          FailureReason = "no_reply"
	FailureGithubPostFailed FailureReason = "github_post_failed"
	FailureAccountError     FailureReason = "account_error"
	FailureSessionRevoked   FailureReason = "session_revoked"
	FailureRunFailed        FailureReason = "run_failed"
)

func (reason FailureReason) IsValid() bool {
	switch reason {
	case FailureRunnerRevoked, FailureRunnerLost, FailureTimeout, FailureSandboxLimit, FailureAdapterError, FailureNoRepository,
		FailureNoReply, FailureGithubPostFailed, FailureAccountError, FailureSessionRevoked, FailureRunFailed:
		return true
	}
	return false
}

const NoReplyTimeout = 24 * time.Hour

var (
	ErrRunNotFound      = httpapi.NewAPIError(http.StatusNotFound, "run_not_found")
	ErrRunAlreadyActive = httpapi.NewAPIError(http.StatusConflict, "run_already_active")
	ErrRunNotActive     = httpapi.NewAPIError(http.StatusConflict, "run_not_active")
)

type PullRequestRef struct {
	Repository string `json:"repository"`
	Number     int    `json:"number"`
	URL        string `json:"url"`
	Title      string `json:"title,omitempty"`
}

// RunSpec is what a run needs besides the bot's own configuration. It is stored with the run and holds no secret: the
// bot's skills, servers, and the account key are read again when a runner claims the run.
type RunSpec struct {
	Prompt       string           `json:"prompt"`
	Repository   string           `json:"repository,omitempty"`
	Branch       string           `json:"branch,omitempty"`
	TaskKey      string           `json:"taskKey,omitempty"`
	TaskTitle    string           `json:"taskTitle,omitempty"`
	PullRequests []PullRequestRef `json:"pullRequests,omitempty"`
	// Skills records the version of each skill the run started with.
	Skills map[string]int `json:"skills,omitempty"`
}

type Run struct {
	ID                 int64
	Token              string
	ConversationID     int64
	ConversationToken  string
	BotID              int64
	Kind               RunKind
	State              RunState
	FailureReason      *FailureReason
	RequesterUserToken string
	RunnerID           *int64
	CLISessionID       *string
	Spec               RunSpec
	WaitingSince       *time.Time
	TimeoutAt          *time.Time
	CreatedAt          time.Time
	ClaimedAt          *time.Time
	EndedAt            *time.Time
}

func (run Run) IsActive() bool {
	return run.State.IsActive()
}
