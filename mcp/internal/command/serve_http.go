package command

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/remote"
)

// RunServeHTTP serves MCP over Streamable HTTP behind the gateway at /mcp; each request carries its user's token,
// so the server holds no credentials of its own (design §12).
func RunServeHTTP(ctx context.Context, environment Environment) error {
	config, err := remote.LoadHTTPServerConfig(environment.Getenv)
	if err != nil {
		return err
	}
	handler := remote.NewHandler(config, remote.HandlerDependencies{
		Version: Version, HTTPClient: environment.HTTPClient, Toolsets: Toolsets, SyncInterval: ToolsetSyncInterval, Now: time.Now,
	})
	server := &http.Server{
		Addr: config.Address, Handler: handler,
		ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 2 * time.Minute,
	}
	failures := make(chan error, 1)
	go func() {
		slog.Info("genwork-mcp listening", "addr", config.Address, "version", Version)
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			failures <- err
		}
	}()
	select {
	case err := <-failures:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}
