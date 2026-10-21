package command

import (
	"context"
	"log/slog"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
)

func RunMigrate(ctx context.Context, getenv platform.Getenv, schema string) error {
	databaseURL, err := platform.LoadDatabaseURL(getenv)
	if err != nil {
		return err
	}
	if err := platform.Migrate(ctx, databaseURL, schema); err != nil {
		return err
	}
	slog.Info("migrations applied", "schema", schema)
	return nil
}
