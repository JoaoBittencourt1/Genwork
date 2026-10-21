package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/toolset"
)

type capabilitiesResponse struct {
	Modules []struct {
		ID string `json:"id"`
	} `json:"modules"`
}

type meResponse struct {
	Permissions []string `json:"permissions"`
}

// CoreAccessReader asks core which modules are enabled and which permissions the signed-in user holds.
type CoreAccessReader struct {
	api platform.APIRequester
}

func NewCoreAccessReader(api platform.APIRequester) *CoreAccessReader {
	return &CoreAccessReader{api: api}
}

func (reader *CoreAccessReader) ReadAccess(ctx context.Context) (toolset.AccessSnapshot, error) {
	var capabilities capabilitiesResponse
	if err := reader.fetch(ctx, "/api/core/v1/capabilities", &capabilities); err != nil {
		return toolset.AccessSnapshot{}, err
	}
	var me meResponse
	if err := reader.fetch(ctx, "/api/core/v1/me", &me); err != nil {
		return toolset.AccessSnapshot{}, err
	}
	snapshot := toolset.AccessSnapshot{Permissions: me.Permissions}
	for _, module := range capabilities.Modules {
		snapshot.EnabledModuleIDs = append(snapshot.EnabledModuleIDs, module.ID)
	}
	return snapshot, nil
}

func (reader *CoreAccessReader) fetch(ctx context.Context, path string, target any) error {
	raw, err := reader.api.Send(ctx, platform.APIRequest{Method: http.MethodGet, Path: path})
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("%w: decode %s: %v", platform.ErrInstanceUnavailable, path, err)
	}
	return nil
}
