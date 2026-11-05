package tasks

import (
	"context"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/coreclient"
)

// AssigneeKind says whether a task's assignee is a person or a bot member.
type AssigneeKind string

const (
	AssigneeKindHuman AssigneeKind = "human"
	AssigneeKindBot   AssigneeKind = "bot"
)

// ParseAssigneeKind reads core's member kind; anything but "bot" is a person, as core's directory predates bots.
func ParseAssigneeKind(coreKind string) AssigneeKind {
	if coreKind == string(AssigneeKindBot) {
		return AssigneeKindBot
	}
	return AssigneeKindHuman
}

// UserDirectory confirms assignees through core's user directory; tests use coretest's fake core (rule 25).
type UserDirectory interface {
	FindUser(ctx context.Context, accessToken, userToken string) (coreclient.DirectoryUser, error)
}
