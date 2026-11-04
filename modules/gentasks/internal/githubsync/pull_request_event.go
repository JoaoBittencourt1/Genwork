package githubsync

import (
	"slices"
	"time"
)

// PullRequestAction is the pull_request webhook "action" field; only these values change snapshots, links, or tasks.
type PullRequestAction string

const (
	ActionOpened           PullRequestAction = "opened"
	ActionReopened         PullRequestAction = "reopened"
	ActionEdited           PullRequestAction = "edited"
	ActionReadyForReview   PullRequestAction = "ready_for_review"
	ActionConvertedToDraft PullRequestAction = "converted_to_draft"
	ActionReviewRequested  PullRequestAction = "review_requested"
	ActionClosed           PullRequestAction = "closed"
)

var handledPullRequestActions = []PullRequestAction{
	ActionOpened, ActionReopened, ActionEdited, ActionReadyForReview, ActionConvertedToDraft, ActionReviewRequested, ActionClosed,
}

func (action PullRequestAction) IsHandled() bool {
	return slices.Contains(handledPullRequestActions, action)
}

type PullRequestEvent struct {
	Action     PullRequestAction
	Repository string
	Number     int
	Title      string
	HeadRef    string
	BaseRef    string
	Merged     bool
	State      PullRequestState
	Author     PullRequestAuthor
	UpdatedAt  time.Time
}

func (event PullRequestEvent) Snapshot() PullRequestSnapshot {
	return PullRequestSnapshot{
		Repository: event.Repository, Number: event.Number, Title: event.Title, State: event.State, Author: event.Author,
		GithubUpdatedAt: event.UpdatedAt,
	}
}

// Trigger maps the event to the board setting it uses; found is false for events that never move a task.
func (event PullRequestEvent) Trigger() (trigger PullRequestTrigger, found bool) {
	switch event.Action {
	case ActionOpened, ActionReopened, ActionEdited, ActionReadyForReview:
		return TriggerOpened, true
	case ActionReviewRequested:
		return TriggerReviewRequested, true
	case ActionClosed:
		return TriggerMerged, event.Merged
	default:
		return "", false
	}
}

// TargetColumn applies the movement rules (design §9): nothing moves a task a merge already moved, merges count only on
// final branches, an unconfigured trigger moves nothing, and tasks only ever move to a column further right.
func (event PullRequestEvent) TargetColumn(task CitedTask, trigger PullRequestTrigger, settings BoardGithubSettings, mergeLocked bool) *MappedColumn {
	if mergeLocked || (trigger == TriggerMerged && !settings.IsFinalBranch(event.BaseRef)) {
		return nil
	}
	target := settings.ColumnFor(trigger)
	if target == nil || target.Position <= task.ColumnPosition {
		return nil
	}
	return target
}
