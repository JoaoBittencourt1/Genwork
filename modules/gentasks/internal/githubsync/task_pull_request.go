package githubsync

import "time"

// TaskPullRequest links a task to a pull request that cites it. Automated moves carry no user, so the move columns are
// how they are attributed to Genwork; they stay nil until the pull request moves the task, and MergeMovedAt locks the
// task against further automated moves even after the link is removed.
type TaskPullRequest struct {
	ID             int64
	Token          string
	TaskID         int64
	PullRequestID  int64
	LastMovedEvent *PullRequestTrigger
	MovedAt        *time.Time
	MergeMovedAt   *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

type TaskPullRequestMove struct {
	TaskID        int64
	PullRequestID int64
	Trigger       PullRequestTrigger
	At            time.Time
}

type LinkedPullRequest struct {
	Link        TaskPullRequest
	PullRequest GithubPullRequest
}
