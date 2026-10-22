package remote

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/core"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/toolset"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// sessionServerFactory builds one MCP server per session, whose tools follow that session's user.
type sessionServerFactory struct {
	dependencies HandlerDependencies
	gatewayURL   string
}

// newSessionServer must not call anything: go-sdk calls it for every request, and keeps the server only when the
// request opens a session. The tools are read when that session's initialize arrives.
func (factory sessionServerFactory) newSessionServer(*http.Request) *mcp.Server {
	server := toolset.NewServer(factory.dependencies.Version)
	api := platform.NewAuthorizedAPIClient(platform.NewAPIClient(factory.gatewayURL, factory.dependencies.HTTPClient), requestAccessTokenSource{})
	synchronizer := toolset.NewToolsetSynchronizer(server, core.NewCoreAccessReader(api), factory.dependencies.Toolsets(api)...)
	resync := &lazyToolsetSync{synchronizer: synchronizer, interval: factory.dependencies.SyncInterval, now: factory.dependencies.Now}
	server.AddReceivingMiddleware(resync.withRequestToken)
	return server
}

// lazyToolsetSync re-reads capabilities and permissions when a request arrives after the interval: between requests
// there is no token to read them with, so the stdio server's timer cannot work here (design §12).
type lazyToolsetSync struct {
	synchronizer *toolset.ToolsetSynchronizer
	interval     time.Duration
	now          func() time.Time
	mutex        sync.Mutex
	syncedAt     time.Time
}

func (resync *lazyToolsetSync) withRequestToken(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
		if extra := request.GetExtra(); extra != nil {
			ctx = withAccessToken(ctx, accessTokenOf(extra.TokenInfo))
		}
		resync.syncIfStale(ctx)
		return next(ctx, method, request)
	}
}

// syncIfStale keeps the previous tools when a sync fails, and tries again with the next request.
func (resync *lazyToolsetSync) syncIfStale(ctx context.Context) {
	resync.mutex.Lock()
	defer resync.mutex.Unlock()
	now := resync.now()
	if !resync.syncedAt.IsZero() && now.Sub(resync.syncedAt) < resync.interval {
		return
	}
	if err := resync.synchronizer.Sync(ctx); err != nil {
		slog.WarnContext(ctx, "read the tools available to the user", "err", err)
		return
	}
	resync.syncedAt = now
}
