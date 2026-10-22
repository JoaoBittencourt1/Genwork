package githubsync

import "time"

// TaskPullRequest records the last time a pull request moved a task. Automated moves carry no user, so these rows are
// how they are attributed to Genwork; MergeMovedAt locks the task against further automated moves.
type TaskPullRequest struct {
	ID                int64
	Token             string
	TaskID            int64
	Repository        string
	PullRequestNumber int
	LastMovedEvent    PullRequestTrigger
	MovedAt           time.Time
	MergeMovedAt      *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type TaskPullRequestMove struct {
	TaskID            int64
	Repository        string
	PullRequestNumber int
	Trigger           PullRequestTrigger
	At                time.Time
}
