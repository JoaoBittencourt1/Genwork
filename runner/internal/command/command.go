// Package command is the genwork-runner command line: register, run, account login, and version.
package command

import (
	"context"
	"fmt"
	"io"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/config"
)

// Version is the platform version, injected at build time (rule 50); a build without it reports 0.0.0-dev.
var Version = "0.0.0-dev"

const usage = `usage: genwork-runner <command>

commands:
  register       exchange a registration token for this runner's secret (--instance URL --token TOKEN)
  run            claim and execute runs (--directory-mode runs without a sandbox, for development only)
  account login  sign a subscription account in, inside a sandbox that mounts only that account's profile
  version        print the version`

func Run(ctx context.Context, args []string, getenv config.Getenv, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("%s", usage)
	}
	switch args[0] {
	case "register":
		return runRegister(ctx, args[1:], getenv, stdout)
	case "run":
		return runServe(ctx, args[1:], getenv, ServeOptions{})
	case "account":
		if len(args) >= 2 && args[1] == "login" {
			return runAccountLogin(ctx, args[2:], getenv, stdout)
		}
		return fmt.Errorf("unknown account command\n\n%s", usage)
	case "version":
		_, err := fmt.Fprintln(stdout, Version)
		return err
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], usage)
	}
}
