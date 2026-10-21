package tasks

import (
	"strconv"
	"time"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

const (
	DefaultAssignedTasksLimit = 20
	MaxAssignedTasksLimit     = 50
)

type CreateTaskRequest struct {
	Title         string   `json:"title"`
	Description   *string  `json:"description"`
	ColumnToken   *string  `json:"columnToken"`
	AssigneeToken *string  `json:"assigneeToken"`
	LabelTokens   []string `json:"labelTokens"`
	Urgency       *Urgency `json:"urgency"`
}

type CreateTaskInput struct {
	Title         string
	Description   *string
	ColumnToken   *string
	AssigneeToken *string
	LabelTokens   []string
	Urgency       Urgency
}

func (request CreateTaskRequest) Validate() (CreateTaskInput, error) {
	fields := map[string]string{}
	input := CreateTaskInput{
		Title:         NormalizeTitle(request.Title),
		Description:   request.Description,
		ColumnToken:   request.ColumnToken,
		AssigneeToken: request.AssigneeToken,
		LabelTokens:   uniqueTokens(request.LabelTokens),
		Urgency:       UrgencyNone,
	}
	if !IsValidTitle(input.Title) {
		fields["title"] = "invalid"
	}
	if input.Description != nil && !IsValidDescription(*input.Description) {
		fields["description"] = "too_long"
	}
	if request.Urgency != nil {
		input.Urgency = *request.Urgency
		if !input.Urgency.IsValid() {
			fields["urgency"] = "invalid"
		}
	}
	if len(fields) > 0 {
		return CreateTaskInput{}, httpapi.NewValidationError(fields)
	}
	return input, nil
}

type UpdateTaskRequest struct {
	Title         httpapi.PatchField[string]   `json:"title"`
	Description   httpapi.PatchField[string]   `json:"description"`
	AssigneeToken httpapi.PatchField[string]   `json:"assigneeToken"`
	Urgency       httpapi.PatchField[Urgency]  `json:"urgency"`
	LabelTokens   httpapi.PatchField[[]string] `json:"labelTokens"`
}

// TaskChanges keeps the PATCH semantics of rule 17: a nil pointer means "leave unchanged".
type TaskChanges struct {
	Title          *string
	Description    *NullableString
	AssigneeToken  *NullableString
	Urgency        *Urgency
	LabelTokens    []string
	ReplacesLabels bool
}

// NullableString is a requested value for a nullable column; Value nil clears it.
type NullableString struct {
	Value *string
}

func (request UpdateTaskRequest) Validate() (TaskChanges, error) {
	fields := map[string]string{}
	var changes TaskChanges
	if request.Title.Present {
		title := NormalizeTitle(request.Title.Value)
		if request.Title.Null || !IsValidTitle(title) {
			fields["title"] = "invalid"
		}
		changes.Title = &title
	}
	if request.Description.Present {
		changes.Description = nullableFrom(request.Description)
		if request.Description.HasValue() && !IsValidDescription(request.Description.Value) {
			fields["description"] = "too_long"
		}
	}
	if request.AssigneeToken.Present {
		changes.AssigneeToken = nullableFrom(request.AssigneeToken)
	}
	if request.Urgency.Present {
		if request.Urgency.Null || !request.Urgency.Value.IsValid() {
			fields["urgency"] = "invalid"
		}
		changes.Urgency = &request.Urgency.Value
	}
	if request.LabelTokens.Present {
		if request.LabelTokens.Null {
			fields["labelTokens"] = "invalid"
		}
		changes.ReplacesLabels = true
		changes.LabelTokens = uniqueTokens(request.LabelTokens.Value)
	}
	if len(fields) > 0 {
		return TaskChanges{}, httpapi.NewValidationError(fields)
	}
	return changes, nil
}

// ApplyTo returns the task with the changes made at the given time; only a new assignee restarts the assignment time.
func (changes TaskChanges) ApplyTo(task Task, at time.Time) Task {
	if changes.Title != nil {
		task.Title = *changes.Title
	}
	if changes.Description != nil {
		task.Description = changes.Description.Value
	}
	if changes.AssigneeToken != nil {
		task.AssignedAt = assignmentTime(task, changes.AssigneeToken.Value, at)
		task.AssigneeUserToken = changes.AssigneeToken.Value
	}
	if changes.Urgency != nil {
		task.Urgency = *changes.Urgency
	}
	return task
}

// NewAssigneeToken returns the assignee a PATCH sets, or nil when it leaves or clears it.
func (changes TaskChanges) NewAssigneeToken() *string {
	if changes.AssigneeToken == nil {
		return nil
	}
	return changes.AssigneeToken.Value
}

func assignmentTime(task Task, assigneeToken *string, at time.Time) *time.Time {
	switch {
	case assigneeToken == nil:
		return nil
	case task.AssigneeUserToken != nil && *task.AssigneeUserToken == *assigneeToken:
		return task.AssignedAt
	default:
		return &at
	}
}

// ListAssignedTasksRequest holds the raw query string; an empty Limit takes the default.
type ListAssignedTasksRequest struct {
	Limit string
}

func (request ListAssignedTasksRequest) Validate() (int, error) {
	if request.Limit == "" {
		return DefaultAssignedTasksLimit, nil
	}
	limit, err := strconv.Atoi(request.Limit)
	if err != nil || limit < 1 || limit > MaxAssignedTasksLimit {
		return 0, httpapi.NewValidationError(map[string]string{"limit": "invalid"})
	}
	return limit, nil
}

type MoveTaskRequest struct {
	ColumnToken string `json:"columnToken"`
	Position    *int   `json:"position"`
}

func (request MoveTaskRequest) Validate() (string, int, error) {
	fields := map[string]string{}
	if request.ColumnToken == "" {
		fields["columnToken"] = "required"
	}
	if request.Position == nil || *request.Position < 0 {
		fields["position"] = "invalid"
	}
	if len(fields) > 0 {
		return "", 0, httpapi.NewValidationError(fields)
	}
	return request.ColumnToken, *request.Position, nil
}

func nullableFrom(field httpapi.PatchField[string]) *NullableString {
	if field.Null {
		return &NullableString{}
	}
	value := field.Value
	return &NullableString{Value: &value}
}

func uniqueTokens(tokens []string) []string {
	seen := map[string]bool{}
	unique := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if !seen[token] {
			seen[token] = true
			unique = append(unique, token)
		}
	}
	return unique
}
