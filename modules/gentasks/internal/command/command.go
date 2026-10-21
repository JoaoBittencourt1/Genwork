package command

import (
	"context"
	"fmt"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
)

const usage = `usage: gentasks <command>

commands:
  serve     run the HTTP API and register with core
  migrate   apply database migrations`

func Run(ctx context.Context, args []string, getenv platform.Getenv) error {
	if len(args) == 0 {
		return fmt.Errorf("%s", usage)
	}
	switch args[0] {
	case "serve":
		return RunServe(ctx, getenv, platform.Schema)
	case "migrate":
		return RunMigrate(ctx, getenv, platform.Schema)
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], usage)
	}
}
