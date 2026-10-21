package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/core"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/gentasks"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/toolset"
)

const ToolsetSyncInterval = 5 * time.Minute

// RunServe fails at startup when the user is not signed in or the instance is unreachable, so the assistant
// shows the reason instead of a server without tools.
func RunServe(ctx context.Context, environment Environment) error {
	config, err := platform.LoadConfig(environment.Getenv)
	if err != nil {
		return err
	}
	sessions, err := newSessionService(config, environment)
	if err != nil {
		return err
	}
	if _, err := sessions.SignedInEmail(); err != nil {
		return err
	}
	api := platform.NewAuthorizedAPIClient(platform.NewAPIClient(config.InstanceURL, environment.HTTPClient), sessions)
	server := toolset.NewServer(Version)
	synchronizer := toolset.NewToolsetSynchronizer(server, core.NewCoreAccessReader(api), Toolsets(api)...)
	if err := synchronizer.Sync(ctx); err != nil {
		return fmt.Errorf("read the tools available to you: %w", err)
	}
	go synchronizer.SyncEvery(ctx, ToolsetSyncInterval, func(err error) {
		fmt.Fprintln(environment.Stderr, "genwork-mcp: refresh the available tools:", err)
	})
	err = server.Run(ctx, environment.ServerTransport)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

// Toolsets lists core and every module this server knows; a module's tools show only while it is enabled.
func Toolsets(api platform.APIRequester) []toolset.Toolset {
	return []toolset.Toolset{
		core.NewToolset(api),
		gentasks.NewToolset(api),
	}
}
