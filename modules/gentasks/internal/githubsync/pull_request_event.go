package githubsync

// PullRequestAction is the pull_request webhook "action" field; only these values can move a task.
type PullRequestAction string

const (
	ActionOpened          PullRequestAction = "opened"
	ActionReopened        PullRequestAction = "reopened"
	ActionEdited          PullRequestAction = "edited"
	ActionReadyForReview  PullRequestAction = "ready_for_review"
	ActionReviewRequested PullRequestAction = "review_requested"
	ActionClosed          PullRequestAction = "closed"
)

type PullRequestEvent struct {
	Action     PullRequestAction
	Repository string
	Number     int
	Title      string
	HeadRef    string
	BaseRef    string
	Merged     bool
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
