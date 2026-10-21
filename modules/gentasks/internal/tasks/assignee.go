package tasks

import (
	"context"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/coreclient"
)

// UserDirectory confirms assignees through core's user directory; tests use coretest's fake core (rule 25).
type UserDirectory interface {
	FindUser(ctx context.Context, accessToken, userToken string) (coreclient.DirectoryUser, error)
}
