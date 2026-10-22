package command

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/JoaoBittencourt1/Genwork/modules/genmonitor/internal/platform"
)

const usage = `usage: genmonitor <command>

commands:
  serve     sample the host, run the HTTP API, and register with core
  migrate   apply database migrations (genmonitor has none)`

func Run(ctx context.Context, args []string, getenv platform.Getenv) error {
	if len(args) == 0 {
		return fmt.Errorf("%s", usage)
	}
	switch args[0] {
	case "serve":
		return RunServe(ctx, getenv)
	case "migrate":
		return RunMigrate()
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], usage)
	}
}

// RunMigrate exists because the installer runs every module's migrate service; genmonitor keeps no data.
func RunMigrate() error {
	slog.Info("genmonitor has no database migrations")
	return nil
}
