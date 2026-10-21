package main

import (
	"context"
	"log/slog"
	"time"
)

func runWorker(ctx context.Context) error {
	slog.Info("worker started", "version", version)
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
