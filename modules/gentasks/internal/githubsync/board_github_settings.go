package githubsync

import (
	"slices"
	"time"
)

// PullRequestTrigger groups the pull request events that can move a task; each maps to one column of the board's settings.
type PullRequestTrigger string

const (
	TriggerOpened          PullRequestTrigger = "opened"
	TriggerReviewRequested PullRequestTrigger = "review_requested"
	TriggerMerged          PullRequestTrigger = "merged"
)

var DefaultFinalBranches = []string{"main", "master"}

// MappedColumn is an active column of the board that a trigger moves tasks to.
type MappedColumn struct {
	ID       int64
	Token    string
	Position int
}

// BoardGithubSettings are the effective settings: while Saved is false they are the defaults — no opened or review
// requested column, the board's done preset column for merges, and DefaultFinalBranches. A deleted column reads as nil.
type BoardGithubSettings struct {
	BoardID         int64
	Saved           bool
	Opened          *MappedColumn
	ReviewRequested *MappedColumn
	Merged          *MappedColumn
	FinalBranches   []string
}

func (settings BoardGithubSettings) ColumnFor(trigger PullRequestTrigger) *MappedColumn {
	switch trigger {
	case TriggerOpened:
		return settings.Opened
	case TriggerReviewRequested:
		return settings.ReviewRequested
	case TriggerMerged:
		return settings.Merged
	default:
		return nil
	}
}

// IsFinalBranch compares case-sensitively, like git does.
func (settings BoardGithubSettings) IsFinalBranch(branch string) bool {
	return slices.Contains(settings.FinalBranches, branch)
}

type SavedBoardGithubSettings struct {
	BoardID                 int64
	OpenedColumnID          *int64
	ReviewRequestedColumnID *int64
	MergedColumnID          *int64
	FinalBranches           []string
	At                      time.Time
}

func (settings BoardGithubSettings) ToSaved(at time.Time) SavedBoardGithubSettings {
	return SavedBoardGithubSettings{
		BoardID:                 settings.BoardID,
		OpenedColumnID:          mappedColumnID(settings.Opened),
		ReviewRequestedColumnID: mappedColumnID(settings.ReviewRequested),
		MergedColumnID:          mappedColumnID(settings.Merged),
		FinalBranches:           settings.FinalBranches,
		At:                      at,
	}
}

func mappedColumnID(column *MappedColumn) *int64 {
	if column == nil {
		return nil
	}
	return &column.ID
}
