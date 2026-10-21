package command

import (
	"context"
	"log/slog"
	"time"
)

func RunWorker(ctx context.Context) error {
	slog.Info("worker started", "version", Version)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("worker stopped")
			return nil
		case <-ticker.C:
		}
	}
}
