package main

import (
	"context"
	"log/slog"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
)

func runMigrate(ctx context.Context, getenv platform.Getenv) error {
	databaseURL, err := platform.LoadDatabaseURL(getenv)
	if err != nil {
		return err
	}
	if err := platform.Migrate(ctx, databaseURL, platform.CoreSchema); err != nil {
		return err
	}
	slog.Info("migrations applied", "schema", platform.CoreSchema)
	return nil
}
