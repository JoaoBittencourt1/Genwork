package tasks

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

const (
	MaxTitleLength       = 200
	MaxDescriptionLength = 10000
)

var (
	ErrTaskNotFound    = httpapi.NewAPIError(http.StatusNotFound, "task_not_found")
	ErrTaskArchived    = httpapi.NewAPIError(http.StatusConflict, "task_archived")
	ErrCoreUnavailable = httpapi.NewAPIError(http.StatusServiceUnavailable, "core_unavailable")
)

// Urgency levels, ordered from lowest to highest.
type Urgency string

const (
	UrgencyNone   Urgency = "none"
	UrgencyLow    Urgency = "low"
	UrgencyMedium Urgency = "medium"
	UrgencyHigh   Urgency = "high"
	UrgencyUrgent Urgency = "urgent"
)

var UrgencyLevels = []Urgency{UrgencyNone, UrgencyLow, UrgencyMedium, UrgencyHigh, UrgencyUrgent}

func (urgency Urgency) IsValid() bool {
	for _, level := range UrgencyLevels {
		if level == urgency {
			return true
		}
	}
	return false
}

// Task.Position is nil once the task is archived: only active tasks take part in the column order.
// Task.Number is unique within its board and, with the board's task prefix, forms the identifier people use.
type Task struct {
	ID                int64
	Token             string
	Number            int
	BoardID           int64
	ColumnID          int64
	ColumnToken       string
	Title             string
	Description       *string
	AssigneeUserToken *string
	Urgency           Urgency
	Position          *int
	CreatedBy         string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	ArchivedAt        *time.Time
}

func (task Task) IsArchived() bool {
	return task.ArchivedAt != nil
}

func NormalizeTitle(title string) string {
	return strings.TrimSpace(title)
}

func IsValidTitle(normalizedTitle string) bool {
	length := utf8.RuneCountInString(normalizedTitle)
	return length > 0 && length <= MaxTitleLength
}

func IsValidDescription(description string) bool {
	return utf8.RuneCountInString(description) <= MaxDescriptionLength
}
