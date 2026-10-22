package command

import (
	"context"
	"fmt"
	"io"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
)

var Version = "0.0.0-dev"

const usage = `usage: core <command>

commands:
  serve          run the HTTP API
  worker         run background jobs
  migrate        apply database migrations
  admin create   create an administrator (--email, --name, --password-stdin)
  module enable  publish a registered module in capabilities (<id>)
  module disable stop publishing a registered module in capabilities (<id>)
  keygen         print a new JWT signing key`

func Run(ctx context.Context, args []string, getenv platform.Getenv, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("%s", usage)
	}
	switch args[0] {
	case "serve":
		return RunServe(ctx, getenv, platform.CoreSchema)
	case "worker":
		return RunWorker(ctx)
	case "migrate":
		return RunMigrate(ctx, getenv, platform.CoreSchema)
	case "admin":
		if len(args) < 2 || args[1] != "create" {
			return fmt.Errorf("%s", usage)
		}
		return RunAdminCreate(ctx, getenv, platform.CoreSchema, args[2:], stdin, stdout)
	case "module":
		return RunModuleState(ctx, getenv, platform.CoreSchema, args[1:], stdout)
	case "keygen":
		return RunKeygen(stdout)
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], usage)
	}
}
