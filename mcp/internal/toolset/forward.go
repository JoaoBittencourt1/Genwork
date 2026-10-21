package toolset

import (
	"context"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform"
)

// NoInput is the input of tools that take no arguments.
type NoInput struct{}

// Forward answers the API body as the tool output, or the confirmation when the API answers no body.
func Forward(ctx context.Context, api platform.APIRequester, request platform.APIRequest, confirmation string) (any, error) {
	body, err := api.Send(ctx, request)
	if err != nil {
		return nil, err
	}
	if body == nil {
		return Confirm(confirmation), nil
	}
	return body, nil
}
